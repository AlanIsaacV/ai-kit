package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Location resolves to one memory log on disk.
type Location struct {
	Scope Scope
	Plan  string // only meaningful for ScopePlan
	Repo  string // absolute repo root, empty for ScopeUser
	Path  string
}

// ErrNoPlan is returned when a plan-scoped operation cannot tell which plan it
// belongs to. Guessing would attach records to the wrong plan, which is worse
// than stopping.
var ErrNoPlan = errors.New("no plan selected: pass --plan <slug> or set AI_KIT_PLAN")

// Resolve locates the log for a scope.
//
//	plan → <repo>/plans/<slug>/memory.jsonl   the working noise of one objective
//	repo → <repo>/.ai-kit/memory.jsonl        distilled, promoted deliberately
//	user → ~/.ai-kit/memory.jsonl             distilled further, across repos
//
// All three are gitignored: these are files the assistant writes for itself,
// and they have no business adding noise to a pull request.
func Resolve(scope Scope, plan, startDir string) (Location, error) {
	loc := Location{Scope: scope, Plan: plan}

	if scope == ScopeUser {
		home, err := os.UserHomeDir()
		if err != nil {
			return loc, fmt.Errorf("resolve home directory: %w", err)
		}
		loc.Path = filepath.Join(home, ".ai-kit", "memory.jsonl")
		return loc, nil
	}

	root, err := RepoRoot(startDir)
	if err != nil {
		return loc, err
	}
	loc.Repo = root

	switch scope {
	case ScopeRepo:
		loc.Path = filepath.Join(root, ".ai-kit", "memory.jsonl")
	case ScopePlan:
		if plan == "" {
			plan = os.Getenv("AI_KIT_PLAN")
		}
		if plan == "" {
			only, err := onlyPlan(root)
			if err != nil {
				return loc, err
			}
			plan = only
		}
		if !slugRE.MatchString(plan) {
			return loc, fmt.Errorf("plan %q must be kebab-case", plan)
		}
		loc.Plan = plan
		loc.Path = filepath.Join(root, "plans", plan, "memory.jsonl")
	default:
		return loc, fmt.Errorf("unknown scope %q; valid: plan, repo, user", scope)
	}
	return loc, nil
}

// onlyPlan returns the single plan under plans/, if there is exactly one.
// With none or several the caller must say which — picking one would silently
// write to the wrong objective's log.
func onlyPlan(root string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "plans"))
	if err != nil {
		return "", ErrNoPlan
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	switch len(names) {
	case 1:
		return names[0], nil
	case 0:
		return "", ErrNoPlan
	default:
		return "", fmt.Errorf("%w (found %d: %v)", ErrNoPlan, len(names), names)
	}
}

// RepoRoot walks up from dir looking for a .git entry.
func RepoRoot(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", dir)
		}
		abs = parent
	}
}
