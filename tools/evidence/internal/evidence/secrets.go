package evidence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	gitleaksBatch     = 16 * 1024
	gitleaksOverlap   = 4 * 1024
	gitleaksLeadWidth = 512

	MaxLine = gitleaksBatch - gitleaksLeadWidth - 2

	reasonOversized = "a line is longer than gitleaks can be relied on to read whole"
	reasonCut       = "a stretch with no blank line in these files is longer than gitleaks can be relied on to read whole, so a secret in them may not be listed: "

	gitleaksChunk   = 96 * 1000
	gitleaksStretch = 24 * 1000
	cutNamesWidth   = 4 * 1024

	scopeTree = "every file of the tree as exported; git history was not scanned"

	gitleaksConfig = "[extend]\nuseDefault = true\n"
	gitleaksRoot   = "tree"
)

var (
	gitleaksVersion = regexp.MustCompile(`(?m)^\s*(v?\d+\.\d+\.\d+\S*)\s*$`)
	gitleaksLead    = strings.Repeat(" ", gitleaksLeadWidth) + "\n"
	errOversized    = errors.New(reasonOversized)
)

type gitleaksLeak struct {
	RuleID    string `json:"RuleID"`
	File      string `json:"File"`
	StartLine int    `json:"StartLine"`
	Secret    string `json:"Secret"`
}

func (s *session) secrets(ctx context.Context, files []fileDiff) Check {
	path, err := lookup(gitleaksName)
	if err != nil {
		return notRun(err.Error())
	}
	version, err := s.scannerVersion(ctx, gitleaksName, path, gitleaksVersion, "version")
	if err != nil {
		return notRun(err.Error())
	}

	read := s.addedSecrets
	var cut []string
	if s.mode == ModeTree {
		read = s.treeSecrets
		cut, err = cutFiles(s.head)
		if err != nil {
			return notRun("the tree could not be walked")
		}
	}
	listed, err := read(ctx, path, files)
	if err != nil {
		return notRun(err.Error())
	}
	sortFindings(listed)
	listed = slices.CompactFunc(listed, func(a, b Finding) bool {
		return a.Path == b.Path && a.Line == b.Line && a.Rule == b.Rule
	})

	check := Check{
		Status:   StatusRan,
		Scanners: []Scanner{newScanner(gitleaksName, version, gitleaksPin, "rules built into the scanner", StatusNotApplicable)},
		New:      listed,
	}
	if len(cut) > 0 {
		check.Status = StatusPartial
		check.Reason = cutReason(cut)
	}
	return check
}

func cutFiles(root string) ([]string, error) {
	var cut []string
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() <= gitleaksChunk+gitleaksStretch {
			return nil
		}
		risky, err := cutAtRisk(name)
		if err != nil || !risky {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		cut = append(cut, filepath.ToSlash(relative))
		return nil
	})
	return cut, err
}

func cutAtRisk(name string) (bool, error) {
	file, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	newlines, last := 0, 0
	for offset := 1; ; offset++ {
		char, err := reader.ReadByte()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch char {
		case '\n':
			newlines++
			if newlines >= 2 {
				last = offset
			}
		case '\r', ' ', '\t':
		default:
			newlines = 0
		}
		if offset-max(last, gitleaksChunk) > gitleaksStretch {
			return true, nil
		}
	}
}

func cutReason(cut []string) string {
	named, width := 0, 0
	for named < len(cut) && (named == 0 || width+len(cut[named]) <= cutNamesWidth) {
		width += len(cut[named]) + 2
		named++
	}
	reason := reasonCut + strings.Join(cut[:named], ", ")
	if named < len(cut) {
		reason += fmt.Sprintf(", and %d more", len(cut)-named)
	}
	return reason
}

func (s *session) addedSecrets(ctx context.Context, path string, files []fileDiff) ([]Finding, error) {
	lines, origins := addedLines(files)
	leaks, err := s.readLines(ctx, path, lines, gitleaksOverlap, false)
	if err != nil {
		return nil, err
	}

	listed := make([]Finding, 0, len(leaks))
	for _, leak := range leaks {
		if leak.Secret != "" {
			s.values = append(s.values, leak.Secret)
		}
		finding := Finding{Rule: leak.RuleID, Path: "added lines", Line: leak.StartLine}
		if leak.StartLine >= 1 && leak.StartLine <= len(origins) {
			origin := origins[leak.StartLine-1]
			finding.Path = origin.path
			finding.Line = origin.line
		}
		listed = append(listed, finding)
	}
	return listed, nil
}

func (s *session) treeSecrets(ctx context.Context, path string, _ []fileDiff) ([]Finding, error) {
	config := filepath.Join(s.work, "gitleaks.toml")
	if err := os.WriteFile(config, []byte(gitleaksConfig), 0o600); err != nil {
		return nil, errors.New("the run could not write its own scanner settings")
	}
	root := filepath.Join(s.work, gitleaksRoot)
	if err := os.Symlink(s.head, root); err != nil {
		return nil, errors.New("the run could not place the tree in its own directory")
	}
	defer os.Remove(root)

	leaks, err := s.gitleaks(ctx, path, nil,
		"dir", root+string(filepath.Separator),
		"--no-banner",
		"--exit-code", "0",
		"--ignore-gitleaks-allow",
		"--config", config,
		"--gitleaks-ignore-path", s.work,
	)
	if err != nil {
		return nil, err
	}

	listed := make([]Finding, 0, len(leaks))
	for _, leak := range leaks {
		if leak.Secret != "" {
			s.values = append(s.values, leak.Secret)
		}
		relative, err := filepath.Rel(root, leak.File)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, errors.New(gitleaksName + " named a file outside the tree")
		}
		listed = append(listed, Finding{Rule: leak.RuleID, Path: filepath.ToSlash(relative), Line: leak.StartLine})
	}
	return listed, nil
}

func (s *session) readLines(ctx context.Context, path string, lines []string, overlap int, redact bool) ([]gitleaksLeak, error) {
	leaks := []gitleaksLeak{}
	for from := 0; from < len(lines); {
		to := batchEnd(lines, from)
		if to == from {
			return nil, errOversized
		}
		found, err := s.readLeaks(ctx, path, batchText(lines[from:to]), redact)
		if err != nil {
			return nil, err
		}
		for _, leak := range found {
			leak.StartLine += from - 1
			leaks = append(leaks, leak)
		}
		from = nextBatch(lines, to, overlap)
	}
	return leaks, nil
}

func batchEnd(lines []string, from int) int {
	size := 0
	to := from
	for to < len(lines) && size+len(lines[to]) <= MaxLine {
		size += len(lines[to]) + 1
		to++
	}
	return to
}

func nextBatch(lines []string, to, overlap int) int {
	if to == len(lines) {
		return to
	}
	room := min(overlap, MaxLine-len(lines[to])-1)
	start := to
	for start > 0 && room >= len(lines[start-1])+1 {
		room -= len(lines[start-1]) + 1
		start--
	}
	return start
}

func batchText(lines []string) []byte {
	var text bytes.Buffer
	text.WriteString(gitleaksLead)
	for _, line := range lines {
		text.WriteString(line)
		text.WriteByte('\n')
	}
	return text.Bytes()
}

func (s *session) readLeaks(ctx context.Context, path string, stream []byte, redact bool) ([]gitleaksLeak, error) {
	args := []string{
		"stdin",
		"--no-banner",
		"--exit-code", "0",
		"--ignore-gitleaks-allow",
	}
	if redact {
		args = append(args, "--redact")
	}
	return s.gitleaks(ctx, path, stream, args...)
}

func (s *session) gitleaks(ctx context.Context, path string, stdin []byte, args ...string) ([]gitleaksLeak, error) {
	report := filepath.Join(s.work, "gitleaks.json")
	args = append(args, "--report-format", "json", "--report-path", report)
	result := s.execute(ctx, s.work, nil, stdin, path, args...)
	raw, unread := os.ReadFile(report)
	os.Remove(report)
	if result.err != nil {
		return nil, errors.New(result.failure(gitleaksName))
	}
	if unread != nil {
		return nil, errors.New(gitleaksName + " left no report")
	}
	var leaks []gitleaksLeak
	if err := json.Unmarshal(raw, &leaks); err != nil || leaks == nil {
		return nil, errors.New(gitleaksName + " printed something unreadable")
	}
	return leaks, nil
}

type lineOrigin struct {
	path string
	line int
}

func addedLines(files []fileDiff) ([]string, []lineOrigin) {
	var lines []string
	var origins []lineOrigin
	for _, file := range files {
		for _, added := range file.added {
			for _, piece := range linePieces(string(added.text)) {
				lines = append(lines, piece)
				origins = append(origins, lineOrigin{path: file.path, line: added.line})
			}
		}
	}
	return lines, origins
}

func linePieces(line string) []string {
	if len(line) <= MaxLine {
		return []string{line}
	}
	var pieces []string
	for start := 0; ; start += MaxLine - gitleaksOverlap {
		end := min(start+MaxLine, len(line))
		pieces = append(pieces, line[start:end])
		if end == len(line) {
			return pieces
		}
	}
}

func recognisableLines(texts ...string) []string {
	seen := map[string]bool{"": true}
	var lines []string
	add := func(line string) {
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	for _, text := range texts {
		for line := range strings.SplitSeq(text, "\n") {
			add(line)
			for _, field := range strings.Fields(line) {
				for at, char := range field {
					if at > 0 && !alphanumeric(char) {
						add(field[:at])
					}
				}
				for _, piece := range strings.FieldsFunc(field, nameSeparator) {
					add(piece)
				}
			}
		}
	}
	return lines
}

func alphanumeric(char rune) bool {
	return char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func nameSeparator(char rune) bool {
	return char == '/' || char == '@' || char == ':'
}
