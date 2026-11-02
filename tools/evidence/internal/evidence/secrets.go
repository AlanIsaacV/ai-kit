package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
)

var (
	gitleaksVersion = regexp.MustCompile(`(?m)^\s*(v?\d+\.\d+\.\d+\S*)\s*$`)
	gitleaksLead    = strings.Repeat(" ", gitleaksLeadWidth) + "\n"
	errOversized    = errors.New(reasonOversized)
)

type gitleaksLeak struct {
	RuleID    string `json:"RuleID"`
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

	lines, origins := addedLines(files)
	leaks, err := s.readLines(ctx, path, lines, gitleaksOverlap, false)
	if err != nil {
		return notRun(err.Error())
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
	sortFindings(listed)
	listed = slices.CompactFunc(listed, func(a, b Finding) bool {
		return a.Path == b.Path && a.Line == b.Line && a.Rule == b.Rule
	})

	return Check{
		Status:   StatusRan,
		Scanners: []Scanner{newScanner(gitleaksName, version, gitleaksPin, "rules built into the scanner", StatusNotApplicable)},
		New:      listed,
	}
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
	report := filepath.Join(s.work, "gitleaks.json")
	args := []string{
		"stdin",
		"--no-banner",
		"--exit-code", "0",
		"--ignore-gitleaks-allow",
		"--report-format", "json",
		"--report-path", report,
	}
	if redact {
		args = append(args, "--redact")
	}
	result := s.execute(ctx, s.work, nil, stream, path, args...)
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
