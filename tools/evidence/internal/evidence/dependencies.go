package evidence

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type advisory struct {
	id        string
	ids       []string
	aliases   []string
	pkg       string
	version   string
	functions []string
}

type scan struct {
	scanner    Scanner
	advisories []advisory
	toolchain  string
}

func (s *session) dependencies(ctx context.Context) Check {
	targets, err := detectTargets(s.head)
	if err != nil {
		if s.mode == ModeTree {
			return notRun("the tree could not be walked")
		}
		return notRun("the exported head could not be walked")
	}
	check := Check{Status: StatusNotApplicable, Scanners: []Scanner{}, New: []Finding{}}
	if len(targets) == 0 {
		check.Reason = "no dependency manifest in the repository"
		return check
	}

	inherited := 0
	ran := 0
	seen := map[Scanner]bool{}
	for _, t := range targets {
		report := Target{Ecosystem: t.ecosystem, Path: t.path, Status: StatusNotRun, Reason: t.reason}
		if t.reason != "" {
			check.Targets = append(check.Targets, report)
			continue
		}
		found, inheritedHere, scanner, toolchain, err := s.scanTarget(ctx, t)
		if err != nil {
			report.Reason = err.Error()
			check.Targets = append(check.Targets, report)
			continue
		}
		report.Status = StatusRan
		report.Toolchain = toolchain
		if t.ecosystem == ecosystemGo {
			report.GoMod = goModVersion(filepath.Join(s.head, filepath.FromSlash(t.path)))
		}
		check.Targets = append(check.Targets, report)
		check.New = append(check.New, found...)
		inherited += inheritedHere
		ran++
		if !seen[scanner] {
			seen[scanner] = true
			check.Scanners = append(check.Scanners, scanner)
		}
	}

	slices.SortFunc(check.Scanners, func(a, b Scanner) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Database, b.Database), cmp.Compare(a.Version, b.Version))
	})
	sortFindings(check.New)

	switch ran {
	case 0:
		check.Status = StatusNotRun
		check.Reason = "no dependency target could be scanned"
		return check
	case len(targets):
		check.Status = StatusRan
	default:
		check.Status = StatusPartial
		check.Reason = fmt.Sprintf("%d of %d dependency targets were not scanned", len(targets)-ran, len(targets))
	}
	check.Inherited = &inherited
	return check
}

func (s *session) scanTarget(ctx context.Context, t target) ([]Finding, int, Scanner, string, error) {
	scanTree := s.scanLockfile
	if t.ecosystem == ecosystemGo {
		scanTree = s.scanGoModule
	}

	head, err := scanTree(ctx, s.head, t)
	if err != nil {
		if s.mode == ModeTree {
			return nil, 0, Scanner{}, "", err
		}
		return nil, 0, Scanner{}, "", fmt.Errorf("at head: %w", err)
	}

	var base scan
	if s.mode == ModeChange && len(head.advisories) > 0 && existsAtBase(s.base, t.path) {
		base, err = scanTree(ctx, s.base, t)
		if err != nil {
			return nil, 0, Scanner{}, "", fmt.Errorf("at base: %w", err)
		}
	}

	known := map[string]bool{}
	for _, entry := range base.advisories {
		for _, id := range entry.ids {
			known[entry.pkg+"\x00"+id] = true
		}
	}

	inherited := 0
	findings := []Finding{}
	for _, entry := range head.advisories {
		if slices.ContainsFunc(entry.ids, func(id string) bool { return known[entry.pkg+"\x00"+id] }) {
			inherited++
			continue
		}
		findings = append(findings, Finding{
			Rule:      entry.id,
			Path:      t.path,
			Package:   entry.pkg,
			Version:   entry.version,
			Aliases:   entry.aliases,
			Functions: entry.functions,
		})
	}
	return findings, inherited, head.scanner, head.toolchain, nil
}

func existsAtBase(base, relative string) bool {
	info, err := os.Lstat(filepath.Join(base, filepath.FromSlash(relative)))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return err == nil && info.Mode().IsRegular()
}
