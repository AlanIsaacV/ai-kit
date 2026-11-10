package evidence

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	Version = "0.1.0"

	StatusRan           = "ran"
	StatusPartial       = "partial"
	StatusNotRun        = "not-run"
	StatusNotApplicable = "not-applicable"

	ModeChange = "change"
	ModeTree   = "tree"

	RunBudget = 9 * time.Minute

	scannerTempDir = "tmp"
)

type Report struct {
	Mode         string `json:"mode"`
	Secrets      Check  `json:"secrets"`
	Dependencies Check  `json:"dependencies"`
	Workflows    Check  `json:"workflows"`
	Sweep        Sweep  `json:"sweep"`
}

type Sweep struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Check struct {
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	Scanners  []Scanner `json:"scanners"`
	Targets   []Target  `json:"targets,omitempty"`
	New       []Finding `json:"new"`
	Inherited *int      `json:"inherited,omitempty"`
}

type treeReport struct {
	Mode         string    `json:"mode"`
	Secrets      treeCheck `json:"secrets"`
	Dependencies treeCheck `json:"dependencies"`
	Workflows    treeCheck `json:"workflows"`
	Sweep        Sweep     `json:"sweep"`
}

type treeCheck struct {
	Status   string    `json:"status"`
	Reason   string    `json:"reason,omitempty"`
	Scope    string    `json:"scope,omitempty"`
	Scanners []Scanner `json:"scanners"`
	Targets  []Target  `json:"targets,omitempty"`
	Found    []Finding `json:"found"`
}

type Scanner struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Pinned       string `json:"pinned"`
	AtPin        bool   `json:"at_pin"`
	Database     string `json:"database"`
	DatabaseDate string `json:"database_date"`
}

type Target struct {
	Ecosystem string `json:"ecosystem"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	Toolchain string `json:"toolchain,omitempty"`
	GoMod     string `json:"go_mod,omitempty"`
}

type Finding struct {
	Rule      string   `json:"rule"`
	Path      string   `json:"path"`
	Line      int      `json:"line,omitempty"`
	Package   string   `json:"package,omitempty"`
	Version   string   `json:"version,omitempty"`
	Aliases   []string `json:"aliases,omitempty"`
	Functions []string `json:"functions,omitempty"`
	Detail    string   `json:"detail,omitempty"`
}

type Options struct {
	Tree   string
	Head   string
	Base   string
	Diff   string
	Budget time.Duration
}

type session struct {
	mode   string
	head   string
	base   string
	work   string
	temp   string
	values []string
}

func Run(ctx context.Context, opts Options) (Report, error) {
	mode, head, base, files, err := inputs(opts)
	if err != nil {
		return Report{}, err
	}

	if _, err := lookup(gitleaksName); err != nil {
		report := unswept(err.Error())
		report.Mode = mode
		return report, nil
	}

	work, err := os.MkdirTemp("", "evidence-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(work)
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return Report{}, err
	}

	temp := filepath.Join(work, scannerTempDir)
	if err := os.Mkdir(temp, 0o700); err != nil {
		return Report{}, err
	}

	budget := opts.Budget
	if budget == 0 {
		budget = RunBudget
	}
	scanning, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	s := &session{mode: mode, head: head, base: base, work: work, temp: temp}
	report := Report{
		Mode:         mode,
		Secrets:      s.secrets(scanning, files),
		Dependencies: s.dependencies(scanning),
		Workflows:    s.workflows(files),
	}
	return s.sweep(ctx, report), nil
}

func inputs(opts Options) (string, string, string, []fileDiff, error) {
	if opts.Tree != "" {
		if opts.Head != "" || opts.Base != "" || opts.Diff != "" {
			return "", "", "", nil, errors.New("--tree cannot be combined with --head, --base or --diff")
		}
		tree, err := exportDir("--tree", opts.Tree)
		return ModeTree, tree, "", nil, err
	}
	head, err := exportDir("--head", opts.Head)
	if err != nil {
		return "", "", "", nil, err
	}
	base, err := exportDir("--base", opts.Base)
	if err != nil {
		return "", "", "", nil, err
	}
	raw, err := os.ReadFile(opts.Diff)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("--diff: %w", err)
	}
	files, err := parseDiff(raw)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("--diff: %w", err)
	}
	return ModeChange, head, base, files, nil
}

func Encode(w io.Writer, report Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if report.Mode == ModeTree {
		tree := treeReport{
			Mode:         report.Mode,
			Secrets:      inTree(report.Secrets),
			Dependencies: inTree(report.Dependencies),
			Workflows:    inTree(report.Workflows),
			Sweep:        report.Sweep,
		}
		if report.Secrets.Status == StatusRan || report.Secrets.Status == StatusPartial {
			tree.Secrets.Scope = scopeTree
		}
		return encoder.Encode(tree)
	}
	return encoder.Encode(report)
}

func inTree(check Check) treeCheck {
	found := check.New
	if found == nil {
		found = []Finding{}
	}
	return treeCheck{
		Status:   check.Status,
		Reason:   check.Reason,
		Scanners: check.Scanners,
		Targets:  check.Targets,
		Found:    found,
	}
}

func exportDir(flag, path string) (string, error) {
	if path == "" {
		return "", errors.New(flag + " is required")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: %s is not a directory", flag, path)
	}
	return resolved, nil
}

func notRun(reason string) Check {
	return Check{Status: StatusNotRun, Reason: reason, Scanners: []Scanner{}, New: []Finding{}}
}

func sortFindings(findings []Finding) {
	slices.SortFunc(findings, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Package, b.Package),
			cmp.Compare(a.Version, b.Version),
			cmp.Compare(a.Detail, b.Detail),
		)
	})
}
