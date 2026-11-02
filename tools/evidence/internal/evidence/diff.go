package evidence

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type addedLine struct {
	line int
	text []byte
}

type fileDiff struct {
	path    string
	renamed string
	added   []addedLine
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func parseDiff(raw []byte) ([]fileDiff, error) {
	var files []fileDiff
	var current *fileDiff
	headers := 0

	lines := bytes.Split(raw, []byte("\n"))
	for i := 0; i < len(lines); i++ {
		text := string(lines[i])
		switch {
		case strings.HasPrefix(text, "diff --git "):
			files = append(files, fileDiff{})
			current = &files[len(files)-1]
			headers++
		case current == nil:
			continue
		case strings.HasPrefix(text, "rename from "):
			current.renamed = diffPath(strings.TrimPrefix(text, "rename from "), "")
		case strings.HasPrefix(text, "rename to "):
			current.path = diffPath(strings.TrimPrefix(text, "rename to "), "")
		case strings.HasPrefix(text, "+++ "):
			if path := diffPath(strings.TrimPrefix(text, "+++ "), "b/"); path != "/dev/null" {
				current.path = path
			}
		case strings.HasPrefix(text, "--- "):
			if path := diffPath(strings.TrimPrefix(text, "--- "), "a/"); current.path == "" && path != "/dev/null" {
				current.path = path
			}
		case strings.HasPrefix(text, "@@ "):
			match := hunkHeader.FindStringSubmatch(text)
			if match == nil {
				continue
			}
			i = readHunk(lines, i+1, match, current) - 1
		}
	}
	if headers == 0 && len(bytes.TrimSpace(raw)) > 0 {
		return nil, errors.New("not a git diff: no file header in it")
	}
	return files, nil
}

func readHunk(lines [][]byte, start int, header []string, file *fileDiff) int {
	oldLeft := hunkCount(header[1])
	newLeft := hunkCount(header[3])
	next, _ := strconv.Atoi(header[2])

	i := start
	for ; i < len(lines) && (oldLeft > 0 || newLeft > 0); i++ {
		line := lines[i]
		switch {
		case len(line) == 0:
			oldLeft--
			newLeft--
			next++
		case line[0] == '+':
			file.added = append(file.added, addedLine{line: next, text: line[1:]})
			newLeft--
			next++
		case line[0] == '-':
			oldLeft--
		case line[0] == '\\':
		default:
			oldLeft--
			newLeft--
			next++
		}
	}
	return i
}

func hunkCount(field string) int {
	if field == "" {
		return 1
	}
	count, _ := strconv.Atoi(field)
	return count
}

func diffPath(field, prefix string) string {
	field = strings.TrimSuffix(field, "\t")
	if strings.HasPrefix(field, `"`) {
		if unquoted, err := strconv.Unquote(field); err == nil {
			field = unquoted
		}
	}
	return strings.TrimPrefix(field, prefix)
}
