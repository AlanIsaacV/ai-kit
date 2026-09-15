// Package gitcmd is the whole of the tool's shell-out to the git binary, shared by
// every command under tools/memory/cmd.
//
// It sits under cmd/ on purpose: it is still the CLI layer, just the part of it
// more than one binary needs. internal/memory resolves a repository root by
// looking for a .git entry and never invokes the binary, which is what keeps the
// library testable without one installed. One definition here, rather than a
// copy per binary, is what makes the identity memory-import stores beside an old
// line the same identity memory stamps into a new one.
package gitcmd

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func UserName(dir string) string {
	return output(dir, "config", "user.name")
}

// RepoIdentity is what tells two repositories apart in one log where their
// directory name cannot: `api` in two places is one label.
//
// The remote comes first because it survives the repository being moved, cloned
// or checked out twice on one machine — the absolute path survives none of
// those, and is the fallback rather than the answer for exactly that reason. A
// repository with no remote and no resolvable path gets nothing, and Origin.RepoID
// is omitted; see there for why that is a claim rather than a hole.
//
// Which remote is `origin` and only `origin`. That covers a repository having
// none; it does not cover one having several under other names, which nothing
// here decides between.
func RepoIdentity(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	if url := output(root, "remote", "get-url", "origin"); url != "" {
		return url
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

// output runs one git command in dir and returns its trimmed stdout, empty for
// anything that went wrong. Every caller is filling in a field that is allowed
// to be absent, so there is nothing here for a failure to be reported to: a
// machine without git, a directory that is not a repository and a repository
// that has no answer are the same outcome — the tool does not know.
func output(dir string, args ...string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
