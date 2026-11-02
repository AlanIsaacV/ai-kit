package measure

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
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	SideBase = "base"
	SideHead = "head"

	recipeEntry = "recipe"
	tailLines   = 10
	waitDelay   = 2 * time.Minute
)

type Options struct {
	Repo     string
	Base     string
	Head     string
	Recipe   string
	Runs     int
	Progress io.Writer
}

type sample struct {
	probe  string
	metric string
	unit   string
	better string
	value  float64
}

type runResult struct {
	side     string
	index    int
	exitCode int
	failed   bool
	dir      string
	ignored  int
	samples  []sample
}

func Run(ctx context.Context, opts Options) (Report, error) {
	progress := opts.Progress
	if progress == nil {
		progress = io.Discard
	}
	repo, err := filepath.Abs(opts.Repo)
	if err != nil {
		return Report{}, err
	}
	top, err := git(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return Report{}, fmt.Errorf("%s is not a git repository: %w", repo, err)
	}
	baseCommit, err := resolve(ctx, top, opts.Base)
	if err != nil {
		return Report{}, err
	}
	headCommit, err := resolve(ctx, top, opts.Head)
	if err != nil {
		return Report{}, err
	}
	recipe := filepath.Clean(opts.Recipe)
	if filepath.IsAbs(recipe) || recipe == "." || recipe == ".." || strings.HasPrefix(recipe, ".."+string(filepath.Separator)) {
		return Report{}, fmt.Errorf("--recipe %q must be a directory inside the repository, relative to its root", opts.Recipe)
	}

	root, err := tempOutside("measure-", top)
	if err != nil {
		return Report{}, err
	}
	defer removeTree(root)

	exports := map[string]string{
		SideBase: filepath.Join(root, SideBase),
		SideHead: filepath.Join(root, SideHead),
	}
	if err := export(ctx, top, baseCommit, exports[SideBase], root); err != nil {
		return Report{}, err
	}
	if err := export(ctx, top, headCommit, exports[SideHead], root); err != nil {
		return Report{}, err
	}
	recipeCopy := filepath.Join(root, "recipe")
	source := filepath.Join(exports[SideHead], recipe)
	if info, err := os.Lstat(source); err != nil || !info.IsDir() {
		return Report{}, fmt.Errorf("the head %s has no recipe directory at %s", short(headCommit), recipe)
	}
	if err := copyTree(source, recipeCopy); err != nil {
		return Report{}, fmt.Errorf("copying the recipe: %w", err)
	}
	entry := filepath.Join(recipeCopy, recipeEntry)
	if info, err := os.Stat(entry); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return Report{}, fmt.Errorf("the recipe at %s has no executable %s", recipe, recipeEntry)
	}
	results, err := tempOutside("measure-results-", top)
	if err != nil {
		return Report{}, err
	}

	env := recipeEnv()
	var runs []runResult
	for i := 1; i <= opts.Runs; i++ {
		for _, side := range []string{SideBase, SideHead} {
			if ctx.Err() != nil {
				return Report{}, ctx.Err()
			}
			fmt.Fprintf(progress, "measure: %s %d/%d ... ", side, i, opts.Runs)
			started := time.Now()
			result, err := runRecipe(ctx, entry, recipeCopy, exports[side], filepath.Join(results, fmt.Sprintf("%s-%d", side, i)), env)
			if err != nil {
				fmt.Fprintln(progress, "error")
				return Report{}, err
			}
			result.side, result.index = side, i
			elapsed := time.Since(started).Round(100 * time.Millisecond)
			if result.failed {
				fmt.Fprintf(progress, "failed (exit %d) in %s\n", result.exitCode, elapsed)
			} else {
				fmt.Fprintf(progress, "ok in %s, %d metrics\n", elapsed, len(result.samples))
			}
			runs = append(runs, result)
		}
	}
	if ctx.Err() != nil {
		return Report{}, ctx.Err()
	}

	return buildReport(opts, top, recipe, results, baseCommit, headCommit, runs), nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func resolve(ctx context.Context, top, ref string) (string, error) {
	commit, err := git(ctx, top, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q to a commit: %w", ref, err)
	}
	return commit, nil
}

func tempOutside(pattern, top string) (string, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", err
	}
	if inside(dir, top) {
		removeTree(dir)
		return "", fmt.Errorf("the temporary directory %s is inside the measured repository; point TMPDIR elsewhere", dir)
	}
	return dir, nil
}

func inside(path, top string) bool {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	rel, err := filepath.Rel(top, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func export(ctx context.Context, top, commit, dest, root string) error {
	archive := filepath.Join(root, short(commit)+".tar")
	_, err := git(ctx, top,
		"-c", "filter.lfs.smudge=", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=false",
		"archive", "--format=tar", "--output", archive, commit)
	if err == nil {
		err = os.MkdirAll(dest, 0o755)
	}
	if err == nil {
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "tar", "-xf", archive, "-C", dest)
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr != nil {
			err = fmt.Errorf("%w: %s", runErr, strings.TrimSpace(stderr.String()))
		}
	}
	os.Remove(archive)
	if err != nil {
		return fmt.Errorf("exporting %s: %w", short(commit), err)
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
		return nil
	})
}

func removeTree(dir string) {
	if os.RemoveAll(dir) == nil {
		return
	}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(path, 0o700)
		}
		return nil
	})
	os.RemoveAll(dir)
}

var inheritedEnv = []string{"PATH", "HOME", "USER", "TMPDIR"}

func recipeEnv() []string {
	var env []string
	for _, name := range inheritedEnv {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func runRecipe(ctx context.Context, entry, dir, code, runDir string, env []string) (runResult, error) {
	out := filepath.Join(runDir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return runResult{}, err
	}
	stdoutPath := filepath.Join(runDir, "stdout.log")
	stderrPath := filepath.Join(runDir, "stderr.log")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return runResult{}, err
	}
	defer stdout.Close()
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return runResult{}, err
	}
	defer stderr.Close()

	cmd := exec.CommandContext(ctx, entry, "--code", code, "--out", out)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = waitDelay

	result := runResult{dir: runDir}
	if err := cmd.Run(); err != nil {
		result.failed = true
		result.exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(stderr, "measure: %v\n", err)
		}
	}
	if err := stdout.Sync(); err != nil {
		return runResult{}, err
	}
	samples, ignored, err := parseMetrics(stdoutPath)
	if err != nil {
		return runResult{}, err
	}
	result.samples, result.ignored = samples, ignored
	return result, nil
}

func parseMetrics(path string) ([]sample, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	var samples []sample
	ignored := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if s, ok := parseLine(line); ok {
			samples = append(samples, s)
		} else {
			ignored++
		}
	}
	return samples, ignored, scanner.Err()
}

func parseLine(line []byte) (sample, bool) {
	var raw struct {
		Probe  *string  `json:"probe"`
		Metric *string  `json:"metric"`
		Unit   *string  `json:"unit"`
		Value  *float64 `json:"value"`
		Better *string  `json:"better"`
	}
	if len(line) == 0 || line[0] != '{' || json.Unmarshal(line, &raw) != nil {
		return sample{}, false
	}
	if raw.Probe == nil || *raw.Probe == "" || raw.Metric == nil || *raw.Metric == "" || raw.Unit == nil || raw.Value == nil || raw.Better == nil {
		return sample{}, false
	}
	if *raw.Better != "lower" && *raw.Better != "higher" {
		return sample{}, false
	}
	return sample{probe: *raw.Probe, metric: *raw.Metric, unit: *raw.Unit, better: *raw.Better, value: *raw.Value}, true
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
