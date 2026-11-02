package evidence

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ruleUnpinned      = "unpinned-action"
	ruleCredentials   = "credentials-json"
	rulePullReqTarget = "pull-request-target"

	workflowsDir = ".github/workflows"
	actionsDir   = ".github/actions"
	dockerScheme = "docker://"
)

var (
	usesLine        = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*(.*)$`)
	fullSHA         = regexp.MustCompile(`@(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
	imageDigest     = regexp.MustCompile(`@sha256:[0-9a-fA-F]{64}$`)
	credentialsJSON = regexp.MustCompile(`\bcredentials_json\b`)
	pullReqTarget   = regexp.MustCompile(`\bpull_request_target\b`)
	inlineComment   = regexp.MustCompile(`\s#.*$`)
	anyDigest       = regexp.MustCompile(`@[a-z0-9]+:[0-9a-fA-F]+$`)
	actionName      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._/-]+$`)
	imageName       = regexp.MustCompile(`^[A-Za-z0-9.-]+(?::[0-9]+)?(?:/[A-Za-z0-9._-]+)*$`)
	plainRef        = regexp.MustCompile(`^(?:v?[0-9]+(?:\.[0-9]+)*|latest|main|master)$`)
)

type workflowFinding struct {
	path  string
	line  int
	rule  string
	text  string
	shown string
}

func (s *session) workflows(files []fileDiff) Check {
	head, err := workflowFindings(s.head)
	if err != nil {
		return notRun("the workflow files at head could not be read")
	}
	base, err := workflowFindings(s.base)
	if err != nil {
		return notRun("the workflow files at base could not be read")
	}

	check := Check{
		Status:   StatusRan,
		Scanners: []Scanner{newScanner("evidence", Version, Version, "native rules", StatusNotApplicable)},
		New:      []Finding{},
	}
	if !hasWorkflowFiles(s.head) {
		check.Status = StatusNotApplicable
		check.Reason = "no workflow file at head"
		return check
	}

	renames := map[string]string{}
	added := map[string]map[int]bool{}
	for _, file := range files {
		if file.renamed != "" {
			renames[file.path] = file.renamed
		}
		lines := map[int]bool{}
		for _, line := range file.added {
			lines[line.line] = true
		}
		added[file.path] = lines
	}

	remaining := map[string]int{}
	for _, finding := range base {
		remaining[finding.key(finding.path)]++
	}

	inherited := 0
	var pending []workflowFinding
	for _, finding := range head {
		if added[finding.path][finding.line] {
			pending = append(pending, finding)
			continue
		}
		inherited++
		remaining[finding.key(baseName(renames, finding.path))]--
	}
	for _, finding := range pending {
		key := finding.key(baseName(renames, finding.path))
		if remaining[key] > 0 {
			remaining[key]--
			inherited++
			continue
		}
		check.New = append(check.New, Finding{Rule: finding.rule, Path: finding.path, Line: finding.line, Detail: finding.shown})
	}
	sortFindings(check.New)
	check.Inherited = &inherited
	return check
}

func baseName(renames map[string]string, path string) string {
	if original, ok := renames[path]; ok {
		return original
	}
	return path
}

func (f workflowFinding) key(path string) string {
	return path + "\x00" + f.rule + "\x00" + f.text
}

func hasWorkflowFiles(root string) bool {
	paths, err := workflowFiles(root)
	return err == nil && len(paths) > 0
}

func workflowFindings(root string) ([]workflowFinding, error) {
	paths, err := workflowFiles(root)
	if err != nil {
		return nil, err
	}
	var findings []workflowFinding
	for _, path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		for index, line := range bytes.Split(raw, []byte("\n")) {
			findings = append(findings, lineFindings(path, index+1, string(line))...)
		}
	}
	return findings, nil
}

func workflowFiles(root string) ([]string, error) {
	var paths []string
	for _, dir := range []string{workflowsDir, actionsDir} {
		top := filepath.Join(root, filepath.FromSlash(dir))
		err := filepath.WalkDir(top, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if entry.IsDir() {
				if dir == workflowsDir && path != top {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || !isYAML(entry.Name()) {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

func lineFindings(path string, line int, raw string) []workflowFinding {
	text := strings.TrimSpace(raw)
	if text == "" || strings.HasPrefix(text, "#") {
		return nil
	}
	text = strings.TrimSpace(inlineComment.ReplaceAllString(text, ""))

	var findings []workflowFinding
	if match := usesLine.FindStringSubmatch(text); match != nil {
		ref := strings.Trim(strings.TrimSpace(match[1]), `'"`)
		if ref != "" && !pinned(ref) {
			findings = append(findings, workflowFinding{path: path, line: line, rule: ruleUnpinned, text: text, shown: shownRef(ref)})
		}
	}
	if credentialsJSON.MatchString(text) {
		findings = append(findings, workflowFinding{path: path, line: line, rule: ruleCredentials, text: text})
	}
	if pullReqTarget.MatchString(text) {
		findings = append(findings, workflowFinding{path: path, line: line, rule: rulePullReqTarget, text: text})
	}
	return findings
}

func pinned(ref string) bool {
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
		return true
	}
	if strings.HasPrefix(ref, dockerScheme) {
		return imageDigest.MatchString(ref)
	}
	return fullSHA.MatchString(ref)
}

func shownRef(ref string) string {
	if image, ok := strings.CutPrefix(ref, dockerScheme); ok {
		return shownImage(image)
	}
	name, version, _ := strings.Cut(ref, "@")
	if !actionName.MatchString(name) {
		return ""
	}
	if plainRef.MatchString(version) {
		return name + "@" + version
	}
	return name
}

func shownImage(image string) string {
	image = anyDigest.ReplaceAllString(image, "")
	image = image[strings.LastIndexByte(image, '@')+1:]
	name, tag := image, ""
	if colon := strings.LastIndexByte(image, ':'); colon > strings.LastIndexByte(image, '/') {
		name, tag = image[:colon], image[colon+1:]
	}
	if !imageName.MatchString(name) {
		return ""
	}
	if plainRef.MatchString(tag) {
		return dockerScheme + name + ":" + tag
	}
	return dockerScheme + name
}
