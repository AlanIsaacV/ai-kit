package evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	gitleaksName    = "gitleaks"
	osvScannerName  = "osv-scanner"
	govulncheckName = "govulncheck"

	gitleaksPin    = "8.30.1"
	osvScannerPin  = "2.7.0"
	govulncheckPin = "v1.8.0"

	waitDelay = 5 * time.Second
)

func lookup(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "go", "bin")} {
			candidate := filepath.Join(dir, name)
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%s is not installed: not on PATH, in ~/.local/bin or in ~/go/bin", name)
}

type execution struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
	timedOut bool
}

func (s *session) execute(ctx context.Context, dir string, env []string, stdin []byte, path string, args ...string) execution {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), env...), "TMPDIR="+s.temp, "GOTMPDIR="+s.temp)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd.Process.Pid) }
	cmd.WaitDelay = waitDelay
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if cmd.Process != nil {
		killGroup(cmd.Process.Pid)
	}
	result := execution{stdout: stdout.Bytes(), stderr: stderr.Bytes(), err: err, exitCode: -1}
	result.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	var exit *exec.ExitError
	if err == nil {
		result.exitCode = 0
	} else if errors.As(err, &exit) {
		result.exitCode = exit.ExitCode()
	}
	return result
}

func killGroup(leader int) error {
	err := syscall.Kill(-leader, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func (r execution) failure(name string) string {
	switch {
	case r.timedOut:
		return name + " ran out of time"
	case r.exitCode >= 0:
		return fmt.Sprintf("%s exited with status %d", name, r.exitCode)
	case r.err != nil:
		return name + " could not be run"
	}
	return name + " failed"
}

func newScanner(name, version, pinned, database, date string) Scanner {
	return Scanner{
		Name:         name,
		Version:      version,
		Pinned:       pinned,
		AtPin:        strings.TrimPrefix(version, "v") == strings.TrimPrefix(pinned, "v"),
		Database:     database,
		DatabaseDate: date,
	}
}

func (s *session) scannerVersion(ctx context.Context, name, path string, pattern *regexp.Regexp, args ...string) (string, error) {
	result := s.execute(ctx, s.work, nil, nil, path, args...)
	if result.err != nil {
		return "", errors.New(result.failure(name) + " when asked for its version")
	}
	match := pattern.FindSubmatch(result.stdout)
	if match == nil {
		return "", errors.New(name + " did not state a readable version")
	}
	return string(match[1]), nil
}
