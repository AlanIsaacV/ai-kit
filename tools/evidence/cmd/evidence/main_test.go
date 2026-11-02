package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alanv/ai-kit/tools/evidence/internal/evidence"
)

const (
	childMarker  = "EVIDENCE_TEST_RUN_MAIN"
	budgetMarker = "EVIDENCE_TEST_BUDGET"

	settingsFile = "settings.txt"
	workflowFile = ".github/workflows/ci.yml"
)

func TestMain(m *testing.M) {
	if os.Getenv(childMarker) != "" {
		if limit, err := time.ParseDuration(os.Getenv(budgetMarker)); err == nil {
			budget = limit
		}
		main()
		return
	}
	os.Exit(m.Run())
}

func installed(t *testing.T, name string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range append(filepath.SplitList(os.Getenv("PATH")), filepath.Join(home, ".local", "bin"), filepath.Join(home, "go", "bin")) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skipf("the real %s is not installed, so this rule cannot be exercised here", name)
	return ""
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeStandIn(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func addedFile(path, content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	var diff strings.Builder
	fmt.Fprintf(&diff, "diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, path, path, len(lines))
	for _, line := range lines {
		diff.WriteString("+" + line + "\n")
	}
	return diff.String()
}

func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func entriesUnder(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

type seed struct {
	files       map[string]string
	exported    map[string]string
	base        map[string]string
	scanners    func(stubs string)
	noGitleaks  bool
	ownGitleaks bool
}

func blindGitleaks() string {
	return `if [ "$1" = version ]; then echo 8.30.1; exit 0; fi
` + sweepReport("[]")
}

type fixture struct {
	root  string
	head  string
	base  string
	diff  string
	stubs string
	home  string
	tmp   string
	path  string
	files int
}

func prepare(t *testing.T, s seed) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		root:  root,
		head:  filepath.Join(root, "head"),
		base:  filepath.Join(root, "base"),
		diff:  filepath.Join(root, "change.diff"),
		stubs: t.TempDir(),
		home:  t.TempDir(),
		tmp:   t.TempDir(),
	}
	f.path = f.stubs
	if !s.noGitleaks {
		f.path += string(os.PathListSeparator) + os.Getenv("PATH")
	}
	if err := os.MkdirAll(f.base, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, f.head, s.files)
	writeFiles(t, f.head, s.exported)
	writeFiles(t, f.base, s.base)
	var change strings.Builder
	for _, path := range slices.Sorted(maps.Keys(s.files)) {
		change.WriteString(addedFile(path, s.files[path]))
	}
	if err := os.WriteFile(f.diff, []byte(change.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.noGitleaks && !s.ownGitleaks {
		if err := os.Symlink(installed(t, "gitleaks"), filepath.Join(f.stubs, "gitleaks")); err != nil {
			t.Fatal(err)
		}
	}
	if s.scanners != nil {
		s.scanners(f.stubs)
	}
	f.files = len(filesUnder(t, root))
	return f
}

func (f *fixture) command(t *testing.T, extra ...string) *exec.Cmd {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "--head", f.head, "--base", f.base, "--diff", f.diff)
	command.Env = append([]string{
		childMarker + "=1",
		"PATH=" + f.path,
		"HOME=" + f.home,
		"TMPDIR=" + f.tmp,
		"GOPROXY=off",
	}, extra...)
	return command
}

func (f *fixture) assertNoTrace(t *testing.T, stdout, stderr []byte) {
	t.Helper()
	for _, token := range tokens {
		if bytes.Contains(stdout, []byte(token)) {
			t.Errorf("the secret reached stdout: %s", stdout)
		}
		if bytes.Contains(stderr, []byte(token)) {
			t.Errorf("the secret reached stderr: %s", stderr)
		}
		for _, dir := range []string{f.home, f.tmp} {
			for _, path := range entriesUnder(t, dir) {
				if strings.Contains(path, token) {
					t.Errorf("the secret reached a name the run left behind: %s", path)
				}
				info, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() {
					continue
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(raw, []byte(token)) {
					t.Errorf("the secret reached a file the run left behind: %s", path)
				}
			}
		}
	}
	if left := entriesUnder(t, f.tmp); len(left) != 0 {
		t.Errorf("the run left files behind: %v", left)
	}
	if after := len(filesUnder(t, f.root)); after != f.files {
		t.Errorf("the run wrote into the exports: %d files became %d", f.files, after)
	}
}

func (f *fixture) run(t *testing.T, extra ...string) (evidence.Report, []byte) {
	t.Helper()
	command := f.command(t, extra...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	f.assertNoTrace(t, stdout.Bytes(), stderr.Bytes())
	var report evidence.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return report, stdout.Bytes()
}

func assertSeededFound(t *testing.T, report evidence.Report) {
	t.Helper()
	if !slices.ContainsFunc(report.Secrets.New, func(f evidence.Finding) bool { return f.Path == settingsFile && f.Line == 1 }) {
		t.Fatalf("the seeded secret was not found, so nothing here proves it stayed hidden: %+v", report.Secrets)
	}
}

func secretRun(t *testing.T, s seed) evidence.Report {
	t.Helper()
	report, _ := prepare(t, s).run(t)
	assertSeededFound(t, report)
	return report
}

func targetOutcome(t *testing.T, report evidence.Report, path string) string {
	t.Helper()
	for _, target := range report.Dependencies.Targets {
		if target.Path == path {
			return target.Status + ": " + target.Reason
		}
	}
	t.Fatalf("no target %s in %+v", path, report.Dependencies.Targets)
	return ""
}

func emptyDatabase(t *testing.T) string {
	t.Helper()
	database := t.TempDir()
	writeFiles(t, database, map[string]string{
		"index/db.json":      `{"modified":"2026-10-01T00:00:00Z"}`,
		"index/modules.json": "[]",
		"index/vulns.json":   "[]",
	})
	return database
}

func advisoryDatabase(t *testing.T, symbol string) string {
	t.Helper()
	database := t.TempDir()
	writeFiles(t, database, map[string]string{
		"index/db.json":      `{"modified":"2026-10-01T00:00:00Z"}`,
		"index/modules.json": `[{"path":"example.com/dep","vulns":[{"id":"GO-2026-0001","modified":"2026-10-01T00:00:00Z"}]}]`,
		"index/vulns.json":   `[{"id":"GO-2026-0001","modified":"2026-10-01T00:00:00Z"}]`,
		"ID/GO-2026-0001.json": `{"schema_version":"1.3.1","id":"GO-2026-0001","modified":"2026-10-01T00:00:00Z","published":"2026-10-01T00:00:00Z",` +
			`"affected":[{"package":{"name":"example.com/dep","ecosystem":"Go"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"}]}],` +
			`"ecosystem_specific":{"imports":[{"path":"example.com/dep","symbols":["` + symbol + `"]}]}}]}`,
	})
	return database
}

func vendoredModule(function string) map[string]string {
	return map[string]string{
		"go.mod":                        "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v1.0.0\n",
		"main.go":                       "package main\n\nimport \"example.com/dep\"\n\nfunc main() { dep." + function + "() }\n",
		"vendor/modules.txt":            "# example.com/dep v1.0.0\n## explicit\nexample.com/dep\n",
		"vendor/example.com/dep/dep.go": "package dep\n\nfunc " + function + "() {}\n",
	}
}

func withSettings(files map[string]string) map[string]string {
	files[settingsFile] = settings
	return files
}

func realGovulncheck(t *testing.T, database string) func(string) {
	t.Helper()
	real := installed(t, "govulncheck")
	return func(stubs string) {
		writeStandIn(t, stubs, "govulncheck", "exec '"+real+"' -db 'file://"+database+"' \"$@\"\n")
	}
}

func goSum(module, version string) string {
	hash := "h1:" + strings.Repeat("A", 43) + "="
	return module + " " + version + " " + hash + "\n" + module + " " + version + "/go.mod " + hash + "\n"
}

func goStandIn(findings ...string) string {
	return "printf '%s\\n' '" + strings.Join(append([]string{goConfigLine}, findings...), "' '") + "'\n"
}

func osvStandIn(output string) string {
	return `if [ "$1" = --version ]; then echo 'osv-scanner version: 2.7.0'; exit 0; fi
echo 'Loaded crates.io local db from /nowhere' >&2
printf '%s' '` + output + `'
`
}

func waitForFile(t *testing.T, path string, exited <-chan error) {
	t.Helper()
	deadline := time.After(2 * time.Minute)
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		select {
		case err := <-exited:
			t.Fatalf("the run ended before reaching the point to kill it: %v", err)
		case <-deadline:
			t.Fatal("the run never reached the point to kill it")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func assertGone(t *testing.T, pid int) {
	t.Helper()
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	for range 300 {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("process %d outlived the run, so it can still write", pid)
}

func slowGcc(t *testing.T, mark string) func(string) {
	t.Helper()
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc is not on PATH, so cgo code cannot be loaded here")
	}
	return func(stubs string) {
		writeStandIn(t, stubs, "gcc", `case "$*" in
*main.cgo2.c*) : > "$TMPDIR/cc`+token+`.s"; echo $$ > '`+mark+`'; exec sleep 300 ;;
esac
exec '`+gcc+`' "$@"
`)
	}
}

func cgoModule() map[string]string {
	return withSettings(map[string]string{
		"bound/go.mod":  "module example.com/bound\n\ngo 1.26\n",
		"bound/main.go": "package main\n\n// int x;\nimport \"C\"\n\nconst token = \"" + token + "\"\n\nfunc main() { println(token, C.x) }\n",
	})
}

func cgoFixture(t *testing.T, mark string) *fixture {
	t.Helper()
	govulncheck := realGovulncheck(t, emptyDatabase(t))
	gcc := slowGcc(t, mark)
	return prepare(t, seed{files: cgoModule(), scanners: func(stubs string) {
		govulncheck(stubs)
		gcc(stubs)
	}})
}

func sweepReport(leaks string) string {
	return `while [ $# -gt 0 ]; do
  if [ "$1" = --report-path ]; then shift; printf '%s' '` + leaks + `' > "$1"; fi
  shift
done
`
}

func killedRun(t *testing.T, f *fixture, mark string, signal syscall.Signal) time.Duration {
	t.Helper()
	command := f.command(t)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	waitForFile(t, mark, exited)
	reached := time.Since(started)
	if err := command.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if err := <-exited; !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("a killed run ended with %v, want exit status 1 and cleanup done", err)
	}
	assertGone(t, readPID(t, mark))
	if stdout.Len() != 0 {
		t.Errorf("a killed run printed a report: %s", stdout.String())
	}
	f.assertNoTrace(t, stdout.Bytes(), stderr.Bytes())
	return reached
}

func syntheticToken(prefix string, seed, length int) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("a synthetic seed, not a credential: %d", seed)))
	return prefix + hex.EncodeToString(digest[:])[:length]
}

func syntheticJSON(path string, stubs string, value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	full := filepath.Join(stubs, path)
	if err := os.WriteFile(full, raw, 0o644); err != nil {
		panic(err)
	}
	return full
}

func usesWorkflow(refs ...string) string {
	var workflow strings.Builder
	workflow.WriteString("on: push\njobs:\n  ci:\n    steps:\n")
	for _, ref := range refs {
		workflow.WriteString("      - uses: " + ref + "\n")
	}
	return workflow.String()
}

func details(findings []evidence.Finding) []string {
	shown := make([]string, 0, len(findings))
	for _, finding := range findings {
		shown = append(shown, finding.Detail)
	}
	return shown
}

func targetPaths(check evidence.Check) []string {
	paths := make([]string, 0, len(check.Targets))
	for _, target := range check.Targets {
		paths = append(paths, target.Path)
	}
	return paths
}

func wholeTokens(report evidence.Report, path string) int {
	found := 0
	for _, finding := range report.Secrets.New {
		if finding.Path == path && finding.Rule == "github-pat" {
			found++
		}
	}
	return found
}

func goFinding(frames ...string) string {
	return `{"finding":{"osv":"GO-2026-0001","trace":[` + strings.Join(frames, ",") + `]}}`
}

func goFrame(pkg, function string) string {
	return `{"module":"example.com/dep","version":"v1.0.0","package":"` + pkg + `","function":"` + function + `"}`
}

func sweepPass(stubs, real, body string) string {
	passed := filepath.Join(stubs, "added-lines-read")
	return `case "$*" in
version|*--redact*) exec '` + real + `' "$@" ;;
esac
if [ ! -e '` + passed + `' ]; then : > '` + passed + `'; exec '` + real + `' "$@"; fi
` + body
}

func privateKey() string {
	var key strings.Builder
	key.WriteString("-----BEGIN PRIVATE KEY-----\n")
	for line := range 8 {
		key.WriteString(syntheticToken("", 1000+line, 64) + "\n")
	}
	key.WriteString("-----END PRIVATE KEY-----\n")
	return key.String()
}

var (
	sum          = sha256.Sum256([]byte("a synthetic seed, not a credential"))
	hexed        = hex.EncodeToString(sum[:])
	token        = "ghp_" + hexed[:36]
	versionToken = "glpat-" + hexed[36:56]
	stripeToken  = syntheticToken("sk_live_", 1, 24)
	gcpToken     = syntheticToken("AIza", 2, 35)
	npmToken     = syntheticToken("npm_", 3, 36)
	dashedToken  = "AIza" + "SyB3kQ9xT7mP2" + "-" + "rL5wHc8YzD4fG6jN1vUe0"
	azureToken   = "abc1Q~" + syntheticToken("", 6, 34)
	keywordValue = "Zk3vQ9xT7mP2rL5wHc8Y"
	shapedTokens = []string{stripeToken, gcpToken, npmToken, dashedToken}
	tokens       = append([]string{token, versionToken, azureToken, keywordValue}, shapedTokens...)
	settings     = "token = " + token + "\napi_key = \"" + keywordValue + "\"\n"
)

const (
	goConfigLine = `{"config":{"scanner_version":"v1.8.0","db":"https://vuln.go.dev","db_last_modified":"2026-10-01T00:00:00Z","go_version":"go1.27.3"}}`

	guardHead = "not-run: at head: go.mod or go.sum holds what gitleaks reads as a secret, so the go tool was not run on it"
	guardBase = "not-run: at base: go.mod or go.sum holds what gitleaks reads as a secret, so the go tool was not run on it"

	osvPackageLine = `{"package":{"name":"%s","version":"%s"},"groups":[{"ids":["RUSTSEC-2020-0071"]}]}`
)

func TestSecretValueIsNeverPrinted(t *testing.T) {
	t.Run("scanners failing on the token", func(t *testing.T) {
		goErrors := strings.Join([]string{
			"govulncheck: loading packages: There are errors with the provided package patterns:",
			`./main.go:4:14: cannot use "` + token + `" (untyped string constant) as int value in variable declaration`,
			"./main.go:5:6: undefined: " + token,
			"go.mod:5:1: unknown directive: " + token,
			"replacement directory ../" + token + " does not exist",
		}, "\n")
		osvErrors := "Error during extraction: Cargo.lock: toml: expected '=', got " + token + "\nextraction failed on specified lockfile"
		workflow := "on: push\njobs:\n  ci:\n    steps:\n" +
			"      - uses: docker://user:" + token + "@ghcr.io/x/y:1\n" +
			"      - uses: https://" + token + "@github.com/x/y@v1\n" +
			"      - uses: actions/checkout@" + token + "\n" +
			"      - uses: \"" + token + "@github.com/x/y@v1\"\n"

		report := secretRun(t, seed{files: map[string]string{
			settingsFile:       settings,
			"go.mod":           "module example.com/app\n\ngo 1.26\n",
			"read/Cargo.lock":  "version = 3\n",
			"clean/Cargo.lock": "version = 3\n",
			workflowFile:       workflow,
		}, scanners: func(stubs string) {
			writeStandIn(t, stubs, "govulncheck", "cat >&2 <<'OUTPUT'\n"+goErrors+"\nOUTPUT\nexit 1\n")
			writeStandIn(t, stubs, "osv-scanner", `if [ "$1" = --version ]; then echo 'osv-scanner version: 2.7.0'; exit 0; fi
case "$*" in
*/read/Cargo.lock*) cat >&2 <<'OUTPUT'
`+osvErrors+`
OUTPUT
exit 127 ;;
esac
echo 'Loaded `+token+` local db from /nowhere' >&2
printf '%s' '{"results":[]}'
`)
		}})

		if len(report.Dependencies.New) != 0 {
			t.Errorf("listed findings although the scanners failed: %+v", report.Dependencies.New)
		}
		goWant := "not-run: at head: govulncheck on go.mod exited with status 1: packages could not be loaded, which is often a private module"
		if got := targetOutcome(t, report, "go.mod"); got != goWant {
			t.Errorf("go.mod = %q, want %q", got, goWant)
		}
		osvWant := "not-run: at head: osv-scanner on read/Cargo.lock exited with status 127: the lockfile could not be read"
		if got := targetOutcome(t, report, "read/Cargo.lock"); got != osvWant {
			t.Errorf("read/Cargo.lock = %q, want %q", got, osvWant)
		}
		if got := targetOutcome(t, report, "clean/Cargo.lock"); !strings.HasPrefix(got, "ran") {
			t.Errorf("clean/Cargo.lock = %q, want it to have run", got)
		}
		want := []string{"docker://ghcr.io/x/y:1", "", "actions/checkout", ""}
		if len(report.Workflows.New) != len(want) {
			t.Fatalf("workflow findings = %+v, want one for each seeded uses line", report.Workflows.New)
		}
		for index, finding := range report.Workflows.New {
			if finding.Detail != want[index] {
				t.Errorf("detail on line %d = %q, want %q", finding.Line, finding.Detail, want[index])
			}
		}
	})

	t.Run("the real govulncheck over a local database", func(t *testing.T) {
		if _, err := exec.LookPath("gcc"); err != nil {
			t.Skip("gcc is not on PATH, so cgo code cannot be loaded here")
		}
		report := secretRun(t, seed{files: map[string]string{
			settingsFile:     settings,
			"broken/go.mod":  "module example.com/broken\n\ngo 1.26\n",
			"broken/main.go": "package main\n\nfunc main() {\n\tvar x int = \"" + token + "\"\n\t_ = x\n}\n",
			"bound/go.mod":   "module example.com/bound\n\ngo 1.26\n",
			"bound/main.go":  "package main\n\n// int x;\nimport \"C\"\n\nconst token = \"" + token + "\"\n\nfunc main() { println(token, C.x) }\n",
		}, scanners: realGovulncheck(t, emptyDatabase(t))})

		brokenWant := "not-run: at head: govulncheck on broken/go.mod exited with status 1: packages could not be loaded, which is often a private module"
		if got := targetOutcome(t, report, "broken/go.mod"); got != brokenWant {
			t.Errorf("broken/go.mod = %q, want %q", got, brokenWant)
		}
		if got := targetOutcome(t, report, "bound/go.mod"); !strings.HasPrefix(got, "ran") {
			t.Errorf("bound/go.mod = %q, want it to have run", got)
		}
	})

	t.Run("a name on a uses line", func(t *testing.T) {
		workflow := "on: push\njobs:\n  ci:\n    steps:\n" +
			"      - uses: actions/" + token + "@v1\n" +
			"      - uses: docker://ghcr.io/" + token + "/y:1\n"
		report := secretRun(t, seed{files: withSettings(map[string]string{workflowFile: workflow})})

		want := []string{"actions/REDACTED@v1", "docker://ghcr.io/REDACTED/y:1"}
		if len(report.Workflows.New) != len(want) {
			t.Fatalf("workflow findings = %+v, want one for each seeded uses line", report.Workflows.New)
		}
		for index, finding := range report.Workflows.New {
			if finding.Detail != want[index] {
				t.Errorf("detail on line %d = %q, want %q", finding.Line, finding.Detail, want[index])
			}
		}
	})

	t.Run("a path", func(t *testing.T) {
		awsKey := "AKIA" + strings.ToUpper(strings.NewReplacer("0", "A", "1", "B", "8", "C", "9", "D").Replace(hexed[:16]))
		odd := "odd\"\\" + token + "\tx"
		report := secretRun(t, seed{
			files: withSettings(map[string]string{
				token + "/Cargo.lock":                 "version = 3\n",
				token + "/go.mod":                     "module example.com/app\n\ngo 1.26\n",
				token + "/main.go":                    "package main\n\nfunc main() {}\n",
				"web/" + token + "/package.json":      "{}\n",
				".github/workflows/" + token + ".yml": "on: push\njobs:\n  ci:\n    steps:\n      - uses: actions/checkout@v4\n",
				"notes-" + token + ".txt":             settings,
			}),
			exported: map[string]string{
				odd + "/package.json":              "{}\n",
				"new\n" + awsKey + "/package.json": "{}\n",
			},
			scanners: func(stubs string) {
				realGovulncheck(t, emptyDatabase(t))(stubs)
				writeStandIn(t, stubs, "osv-scanner", `if [ "$1" = --version ]; then echo 'osv-scanner version: 2.7.0'; exit 0; fi
echo 'extraction failed on specified lockfile' >&2
exit 127
`)
			},
		})

		if got := targetOutcome(t, report, "REDACTED/Cargo.lock"); got != "not-run: at head: osv-scanner on REDACTED/Cargo.lock exited with status 127: the lockfile could not be read" {
			t.Errorf("the reason keeps the path: %q", got)
		}
		if got := targetOutcome(t, report, "REDACTED/go.mod"); !strings.HasPrefix(got, "ran") {
			t.Errorf("REDACTED/go.mod = %q, want it to have run", got)
		}
		targetOutcome(t, report, "web/REDACTED/package.json")
		targetOutcome(t, report, "odd\"\\REDACTED\tx/package.json")
		targetOutcome(t, report, "new\nREDACTED/package.json")
		if !slices.ContainsFunc(report.Workflows.New, func(f evidence.Finding) bool { return f.Path == ".github/workflows/REDACTED.yml" }) {
			t.Errorf("workflow findings = %+v, want the path swept", report.Workflows.New)
		}
		if !slices.ContainsFunc(report.Secrets.New, func(f evidence.Finding) bool { return f.Path == "notes-REDACTED.txt" }) {
			t.Errorf("secret findings = %+v, want the path swept", report.Secrets.New)
		}
		if report.Sweep.Status != evidence.StatusRan {
			t.Errorf("sweep = %+v, want it to have run", report.Sweep)
		}
	})

	t.Run("a lockfile version", func(t *testing.T) {
		output := `{"results":[{"packages":[{"package":{"name":"time","version":"0.1.43-` + token + `"},"groups":[{"ids":["RUSTSEC-2020-0071"],"aliases":["CVE-2020-26235"]}]}]}]}`
		report := secretRun(t, seed{
			files: withSettings(map[string]string{"Cargo.lock": "version = 3\n"}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "osv-scanner", osvStandIn(output))
			},
		})

		if len(report.Dependencies.New) != 1 || report.Dependencies.New[0].Version != "0.1.43-REDACTED" {
			t.Errorf("dependency findings = %+v, want the version swept", report.Dependencies.New)
		}
	})

	t.Run("a vendored module version", func(t *testing.T) {
		finding := `{"finding":{"osv":"GO-2026-0001","trace":[{"module":"example.com/dep","version":"v1.0.0-` + versionToken + `","package":"example.com/dep/pkg","function":"Bad"}]}}`
		report := secretRun(t, seed{
			files: withSettings(map[string]string{"go.mod": "module example.com/app\n\ngo 1.26\n"}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "govulncheck", goStandIn(finding))
			},
		})

		if len(report.Dependencies.New) != 1 || report.Dependencies.New[0].Version != "v1.0.0-REDACTED" {
			t.Errorf("dependency findings = %+v, want the version swept", report.Dependencies.New)
		}
	})

	t.Run("a vendored module version in go.mod", func(t *testing.T) {
		function := "Bad"
		files := vendoredModule(function)
		files["go.mod"] = "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v1.0.0-" + versionToken + "\n"
		files["vendor/modules.txt"] = "# example.com/dep v1.0.0-" + versionToken + "\n## explicit\nexample.com/dep\n"
		report := secretRun(t, seed{files: withSettings(files), scanners: realGovulncheck(t, advisoryDatabase(t, function))})

		if got := targetOutcome(t, report, "go.mod"); got != guardHead {
			t.Errorf("go.mod = %q, want %q", got, guardHead)
		}
	})

	t.Run("a vendored module function", func(t *testing.T) {
		function := "Fbad" + token
		report := secretRun(t, seed{
			files:    withSettings(vendoredModule(function)),
			scanners: realGovulncheck(t, advisoryDatabase(t, function)),
		})

		if got := targetOutcome(t, report, "go.mod"); !strings.HasPrefix(got, "ran") {
			t.Fatalf("go.mod = %q, want it to have run", got)
		}
		if len(report.Dependencies.New) != 1 || !slices.Equal(report.Dependencies.New[0].Functions, []string{"example.com/dep.FbadREDACTED"}) {
			t.Errorf("dependency findings = %+v, want the function swept", report.Dependencies.New)
		}
	})

	t.Run("a go.mod block", func(t *testing.T) {
		blocks := map[string]string{
			"exclude":             "exclude (\ngo v1.0.0-" + versionToken + "\n)\n",
			"replace":             "replace (\ngo v1.0.0-" + versionToken + " => ./x\n)\n",
			"ignore":              "ignore (\ngo\n" + token + "\n)\n",
			"tool":                "tool (\ngo\n" + token + "\n)\n",
			"a version inside it": "require (\ngo 1.21\n)\n",
		}
		for _, name := range slices.Sorted(maps.Keys(blocks)) {
			t.Run(name, func(t *testing.T) {
				manifest := "module example.com/app\n\n" + blocks[name] + "\ngo 1.26 // a note\n"
				if strings.Contains(manifest, token) || strings.Contains(manifest, versionToken) {
					report := secretRun(t, seed{
						files:    withSettings(map[string]string{"go.mod": manifest, "main.go": "package main\n\nfunc main() {}\n"}),
						scanners: realGovulncheck(t, emptyDatabase(t)),
					})
					if got := targetOutcome(t, report, "go.mod"); got != guardHead {
						t.Errorf("go.mod = %q, want %q", got, guardHead)
					}
				}

				f := prepare(t, seed{
					ownGitleaks: true,
					files:       map[string]string{"go.mod": manifest},
					scanners: func(stubs string) {
						writeStandIn(t, stubs, "gitleaks", blindGitleaks())
						writeStandIn(t, stubs, "govulncheck", goStandIn())
					},
				})
				plain, _ := f.run(t)
				if len(plain.Dependencies.Targets) != 1 || plain.Dependencies.Targets[0].GoMod != "1.26" {
					t.Errorf("targets = %+v, want go_mod read from the top-level line alone", plain.Dependencies.Targets)
				}
			})
		}
	})

	t.Run("a go directive that does not read as a version", func(t *testing.T) {
		manifests := map[string]string{
			"across a newline":   "module example.com/app\n\ngo\n" + token + "\n",
			"a token":            "module example.com/app\n\ngo " + token + "\n",
			"not a version":      "module example.com/app\n\ngo banana\n",
			"with a third field": "module example.com/app\n\ngo 1.26 " + token + "\n",
		}
		for _, name := range slices.Sorted(maps.Keys(manifests)) {
			t.Run(name, func(t *testing.T) {
				f := prepare(t, seed{
					ownGitleaks: true,
					files:       map[string]string{"go.mod": manifests[name]},
					scanners: func(stubs string) {
						writeStandIn(t, stubs, "gitleaks", blindGitleaks())
						writeStandIn(t, stubs, "govulncheck", goStandIn())
					},
				})
				report, _ := f.run(t)
				if len(report.Dependencies.Targets) != 1 || report.Dependencies.Targets[0].GoMod != "" {
					t.Errorf("targets = %+v, want no go_mod", report.Dependencies.Targets)
				}
			})
		}
	})

	t.Run("gitleaks absent", func(t *testing.T) {
		ranScanner := filepath.Join(t.TempDir(), "ran")
		f := prepare(t, seed{
			noGitleaks: true,
			files: withSettings(map[string]string{
				"go.mod":              "module example.com/app\n\ngo 1.26\n\nrequire example.com/" + stripeToken + "/sub v1.0.0\n",
				"go.sum":              goSum("example.com/"+stripeToken+"/sub", "v1.0.0"),
				"main.go":             "package main\n\nimport _ \"example.com/" + stripeToken + "/sub\"\n\nfunc main() {}\n",
				token + "/Cargo.lock": "version = 3\n",
				workflowFile:          usesWorkflow("actions/" + token + "@v1"),
			}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "govulncheck", ": > '"+ranScanner+"'\n"+goStandIn())
				writeStandIn(t, stubs, "osv-scanner", ": > '"+ranScanner+"'\n"+osvStandIn(`{"results":[]}`))
				writeStandIn(t, stubs, "go", ": > '"+ranScanner+"'\n")
			},
		})
		report, _ := f.run(t)

		if _, err := os.Stat(ranScanner); err == nil {
			t.Error("a scanner ran although nothing could sweep what it found")
		}
		if report.Secrets.Status != evidence.StatusNotRun || !strings.Contains(report.Secrets.Reason, "not installed") {
			t.Errorf("secrets = %+v, want not-run", report.Secrets)
		}
		if report.Sweep.Status != evidence.StatusNotRun || !strings.Contains(report.Sweep.Reason, "no sweep") {
			t.Errorf("sweep = %+v, want it to say there was none", report.Sweep)
		}
		for name, check := range map[string]evidence.Check{"dependencies": report.Dependencies, "workflows": report.Workflows} {
			if check.Status != evidence.StatusNotRun || check.Reason != report.Sweep.Reason {
				t.Errorf("%s = %+v, want the one reason of a run with no sweep", name, check)
			}
			if len(check.New) != 0 || len(check.Targets) != 0 {
				t.Errorf("%s listed %+v from a report nothing swept", name, check)
			}
		}
	})

	t.Run("a format token before a character its rule does not allow there", func(t *testing.T) {
		var workflow, wantDetails, wantTargets, wantFunctions []string
		exported := map[string]string{}
		var frames []string
		for index, shape := range shapedTokens {
			directory := fmt.Sprintf("web%d", index)
			workflow = append(workflow, "actions/"+shape+"/sub@v1", "actions/"+shape+"@v1", "actions/"+shape+".x@v1")
			wantDetails = append(wantDetails, "actions/REDACTED.x@v1", "actions/REDACTED/sub@v1", "actions/REDACTED@v1")

			exported[directory+"/"+shape+"/package.json"] = "{}\n"
			exported[directory+"/"+shape+"@x/package.json"] = "{}\n"
			exported[directory+"/"+shape+".d/package.json"] = "{}\n"
			wantTargets = append(wantTargets, directory+"/REDACTED.d/package.json", directory+"/REDACTED/package.json", directory+"/REDACTED@x/package.json")

			frames = append(frames, goFrame("example.com/"+shape+"/pkg", "Bad"), goFrame("example.com/"+shape+"@v1/pkg", "Bad"), goFrame("example.com/dep/"+shape, "Bad"))
			wantFunctions = append(wantFunctions, "example.com/REDACTED/pkg.Bad", "example.com/REDACTED@v1/pkg.Bad", "example.com/dep/REDACTED.Bad")
		}
		slices.Sort(wantFunctions)
		slices.Sort(wantDetails)
		wantTargets = append(wantTargets, "go.mod")
		slices.Sort(wantTargets)

		findings := make([]string, 0, len(frames))
		for _, frame := range frames {
			findings = append(findings, goFinding(frame))
		}
		report := secretRun(t, seed{
			files: withSettings(map[string]string{
				"go.mod":     "module example.com/app\n\ngo 1.26\n",
				workflowFile: usesWorkflow(workflow...),
			}),
			exported: exported,
			scanners: func(stubs string) { writeStandIn(t, stubs, "govulncheck", goStandIn(findings...)) },
		})

		got := details(report.Workflows.New)
		slices.Sort(got)
		if !slices.Equal(got, wantDetails) {
			t.Errorf("details = %q, want %q", got, wantDetails)
		}
		paths := targetPaths(report.Dependencies)
		slices.Sort(paths)
		if !slices.Equal(paths, wantTargets) {
			t.Errorf("targets = %q, want %q", paths, wantTargets)
		}
		if len(report.Dependencies.New) != 1 {
			t.Fatalf("dependency findings = %+v, want one", report.Dependencies.New)
		}
		functions := slices.Clone(report.Dependencies.New[0].Functions)
		slices.Sort(functions)
		if !slices.Equal(functions, wantFunctions) {
			t.Errorf("functions = %q, want %q", functions, wantFunctions)
		}
		if report.Sweep.Status != evidence.StatusRan {
			t.Errorf("sweep = %+v, want it to have run", report.Sweep)
		}
	})

	t.Run("a value with a separator in it stays whole", func(t *testing.T) {
		output := `{"results":[{"packages":[` + fmt.Sprintf(osvPackageLine, "time", "1.0.0-"+dashedToken+"-rc1") + `]}]}`
		report := secretRun(t, seed{
			files: withSettings(map[string]string{"Cargo.lock": "version = 3\n"}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "osv-scanner", osvStandIn(output))
			},
		})

		if len(report.Dependencies.New) != 1 || report.Dependencies.New[0].Version != "1.0.0-REDACTED-rc1" {
			t.Errorf("dependency findings = %+v, want the whole value replaced", report.Dependencies.New)
		}
	})

	t.Run("a value its rule wants alone", func(t *testing.T) {
		report := secretRun(t, seed{
			files:    withSettings(map[string]string{}),
			exported: map[string]string{"web/" + azureToken + "/package.json": "{}\n"},
		})

		targetOutcome(t, report, "web/REDACTED/package.json")
	})

	t.Run("a value known only from the keyword beside it", func(t *testing.T) {
		output := `{"results":[{"packages":[` + fmt.Sprintf(osvPackageLine, "time", "1.0.0-"+keywordValue) + `]}]}`
		report := secretRun(t, seed{
			files: withSettings(map[string]string{
				"Cargo.lock": "version = 3\n",
				workflowFile: usesWorkflow("actions/" + keywordValue + "@v1"),
			}),
			exported: map[string]string{"notes/" + keywordValue + "/package.json": "{}\n"},
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "osv-scanner", osvStandIn(output))
			},
		})

		if !slices.ContainsFunc(report.Secrets.New, func(f evidence.Finding) bool { return f.Path == settingsFile && f.Line == 2 }) {
			t.Fatalf("the keyword value was not found, so nothing here proves it stayed hidden: %+v", report.Secrets)
		}
		targetOutcome(t, report, "notes/REDACTED/package.json")
		if len(report.Dependencies.New) != 1 || report.Dependencies.New[0].Version != "1.0.0-REDACTED" {
			t.Errorf("dependency findings = %+v, want the version replaced", report.Dependencies.New)
		}
		if got := details(report.Workflows.New); !slices.Equal(got, []string{"actions/REDACTED@v1"}) {
			t.Errorf("details = %q, want the name replaced", got)
		}
	})

	t.Run("a large report and a large change", func(t *testing.T) {
		const count = 3000
		packages := make([]json.RawMessage, 0, count)
		reported := make([]string, 0, count)
		var added strings.Builder
		for index := range count {
			value := syntheticToken("ghp_", 10000+index, 36)
			reported = append(reported, value)
			packages = append(packages, json.RawMessage(fmt.Sprintf(osvPackageLine, value, "1.0.0")))
			added.WriteString("token = " + syntheticToken("ghp_", 20000+index, 36) + "\n")
		}
		f := prepare(t, seed{
			files: withSettings(map[string]string{"Cargo.lock": "version = 3\n", "many.txt": added.String()}),
			scanners: func(stubs string) {
				listing := syntheticJSON("packages.json", stubs, map[string]any{"results": []map[string]any{{"packages": packages}}})
				writeStandIn(t, stubs, "osv-scanner", `if [ "$1" = --version ]; then echo 'osv-scanner version: 2.7.0'; exit 0; fi
cat '`+listing+`'
`)
			},
		})
		report, stdout := f.run(t)
		assertSeededFound(t, report)

		for _, value := range reported {
			if bytes.Contains(stdout, []byte(value)) {
				t.Fatalf("a value in a large report was printed: %s", value)
			}
		}
		if len(report.Dependencies.New) != count {
			t.Errorf("listed %d packages, want %d", len(report.Dependencies.New), count)
		}
		if found := wholeTokens(report, "many.txt"); found != count {
			t.Errorf("found %d of the %d values on added lines", found, count)
		}
	})

	t.Run("a stream gitleaks reads one byte at first", func(t *testing.T) {
		real := installed(t, "gitleaks")
		const count = 1000
		var added strings.Builder
		for index := range count {
			added.WriteString("token = " + syntheticToken("ghp_", 30000+index, 36) + "\n")
		}
		report := secretRun(t, seed{
			ownGitleaks: true,
			files:       withSettings(map[string]string{"many.txt": added.String()}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "gitleaks", `if [ "$1" = version ]; then exec '`+real+`' "$@"; fi
{ head -c 1; sleep 0.6; cat; } | '`+real+`' "$@"
`)
			},
		})

		if found := wholeTokens(report, "many.txt"); found != count {
			t.Errorf("found %d of the %d values on added lines", found, count)
		}
	})

	t.Run("a private key across two batches", func(t *testing.T) {
		filler := strings.Repeat(strings.Repeat("y", 79)+"\n", evidence.MaxLine/80-2)
		report := secretRun(t, seed{files: withSettings(map[string]string{"z.txt": filler + privateKey()})})

		if !slices.ContainsFunc(report.Secrets.New, func(f evidence.Finding) bool { return f.Rule == "private-key" }) {
			t.Errorf("secret findings = %+v, want the key", report.Secrets.New)
		}
	})

	t.Run("an added line longer than a batch", func(t *testing.T) {
		files := map[string]string{}
		for offset := range 4 {
			padding := strings.Repeat("y", evidence.MaxLine-10*offset-1)
			files[fmt.Sprintf("long%d.txt", offset)] = padding + " " + token + " " + padding + "\n"
		}
		report := secretRun(t, seed{files: withSettings(files)})

		for offset := range 4 {
			path := fmt.Sprintf("long%d.txt", offset)
			if !slices.ContainsFunc(report.Secrets.New, func(f evidence.Finding) bool { return f.Path == path }) {
				t.Errorf("secret findings = %+v, want the value in %s", report.Secrets.New, path)
			}
		}
		if report.Secrets.Status != evidence.StatusRan {
			t.Errorf("secrets = %+v, want it to have run", report.Secrets)
		}
	})

	t.Run("an added line gitleaks would take for a binary", func(t *testing.T) {
		first := []string{"%PDF-1.7"}
		for shift := range 5 {
			first = append(first, strings.Repeat(" ", shift)+strings.Repeat("ustar", 60))
		}
		for _, line := range first {
			secretRun(t, seed{files: withSettings(map[string]string{"a.txt": line + "\n"})})
		}
	})

	t.Run("a report string longer than a batch", func(t *testing.T) {
		name := "actions/" + strings.Repeat("y", evidence.MaxLine) + token
		f := prepare(t, seed{files: withSettings(map[string]string{workflowFile: usesWorkflow(name + "@v1")})})
		report, _ := f.run(t)

		if report.Sweep.Status != evidence.StatusNotRun || !strings.Contains(report.Sweep.Reason, "read whole") {
			t.Errorf("sweep = %+v, want it to say a line was too long", report.Sweep)
		}
		if len(report.Workflows.New) != 0 || len(report.Secrets.New) != 0 {
			t.Errorf("report = %+v, want nothing from a report nothing swept", report)
		}
	})

	t.Run("a go.mod line longer than a batch", func(t *testing.T) {
		ranScanner := filepath.Join(t.TempDir(), "ran")
		f := prepare(t, seed{
			files: withSettings(map[string]string{"go.mod": "module example.com/app\n\ngo 1.26\n\n// " + strings.Repeat("y", evidence.MaxLine) + "\n"}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "govulncheck", ": > '"+ranScanner+"'\n"+goStandIn())
			},
		})
		report, _ := f.run(t)

		if _, err := os.Stat(ranScanner); err == nil {
			t.Error("the go tool ran on a manifest gitleaks could not read whole")
		}
		if got := targetOutcome(t, report, "go.mod"); !strings.Contains(got, "read whole") {
			t.Errorf("go.mod = %q, want it to say a line was too long", got)
		}
	})

	t.Run("go.mod and go.sum before the go tool", func(t *testing.T) {
		function := "Bad"
		blocked := map[string]map[string]string{
			"a required module": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/" + token + " v1.0.0\n",
				"go.sum":  goSum("example.com/"+token, "v1.0.0"),
				"main.go": "package main\n\nimport _ \"example.com/" + token + "\"\n\nfunc main() {}\n",
			},
			"a replacement": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v1.0.0\n\nreplace example.com/dep => example.com/" + token + " v1.0.0\n",
				"go.sum":  goSum("example.com/"+token, "v1.0.0"),
				"main.go": "package main\n\nimport _ \"example.com/dep\"\n\nfunc main() {}\n",
			},
			"a required version": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v1.0.0-" + versionToken + "\n",
				"go.sum":  goSum("example.com/dep", "v1.0.0-"+versionToken),
				"main.go": "package main\n\nimport _ \"example.com/dep\"\n\nfunc main() {}\n",
			},
			"go.sum alone": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n",
				"go.sum":  goSum("example.com/"+token, "v1.0.0"),
				"main.go": "package main\n\nfunc main() {}\n",
			},
			"a name in the middle of a module path": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/" + stripeToken + "/sub v1.0.0\n",
				"go.sum":  goSum("example.com/"+stripeToken+"/sub", "v1.0.0"),
				"main.go": "package main\n\nimport _ \"example.com/" + stripeToken + "/sub\"\n\nfunc main() {}\n",
			},
			"a keyword value as a module name": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/" + keywordValue + " v1.0.0\n",
				"go.sum":  goSum("example.com/"+keywordValue, "v1.0.0"),
				"main.go": "package main\n\nimport _ \"example.com/" + keywordValue + "\"\n\nfunc main() {}\n",
			},
			"a keyword value as a version": {
				"go.mod":  "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v1.0.0-" + keywordValue + "\n",
				"go.sum":  goSum("example.com/dep", "v1.0.0-"+keywordValue),
				"main.go": "package main\n\nimport _ \"example.com/dep\"\n\nfunc main() {}\n",
			},
		}
		for _, name := range slices.Sorted(maps.Keys(blocked)) {
			t.Run("at head: "+name, func(t *testing.T) {
				report := secretRun(t, seed{files: withSettings(blocked[name]), scanners: realGovulncheck(t, emptyDatabase(t))})
				if got := targetOutcome(t, report, "go.mod"); got != guardHead {
					t.Errorf("go.mod = %q, want %q", got, guardHead)
				}
			})
			t.Run("at base: "+name, func(t *testing.T) {
				report := secretRun(t, seed{
					files:    withSettings(vendoredModule(function)),
					base:     blocked[name],
					scanners: realGovulncheck(t, advisoryDatabase(t, function)),
				})
				if got := targetOutcome(t, report, "go.mod"); got != guardBase {
					t.Errorf("go.mod = %q, want %q", got, guardBase)
				}
				if len(report.Dependencies.New) != 0 {
					t.Errorf("listed findings although the base was not scanned: %+v", report.Dependencies.New)
				}
			})
		}
	})

	t.Run("a go.sum hash beside a keyword is not a secret", func(t *testing.T) {
		hash := "h1:" + strings.NewReplacer("+", "a", "/", "b").Replace(base64.StdEncoding.EncodeToString(sum[:]))
		f := prepare(t, seed{
			files: map[string]string{
				"go.mod": "module example.com/app\n\ngo 1.26\n\nrequire golang.org/x/oauth2 v0.18.0\n",
				"go.sum": "golang.org/x/oauth2 v0.18.0 " + hash + "\ngolang.org/x/oauth2 v0.18.0/go.mod " + hash + "\n",
			},
			scanners: func(stubs string) { writeStandIn(t, stubs, "govulncheck", goStandIn()) },
		})
		report, _ := f.run(t)
		if got := targetOutcome(t, report, "go.mod"); !strings.HasPrefix(got, "ran") {
			t.Errorf("go.mod = %q, want it to have run", got)
		}
	})

	t.Run("a go.sum behind a link", func(t *testing.T) {
		f := prepare(t, seed{
			files:    withSettings(map[string]string{"go.mod": "module example.com/app\n\ngo 1.26\n", "main.go": "package main\n\nfunc main() {}\n"}),
			scanners: func(stubs string) { writeStandIn(t, stubs, "govulncheck", goStandIn()) },
		})
		if err := os.Symlink(filepath.Join(f.root, "elsewhere"), filepath.Join(f.head, "go.sum")); err != nil {
			t.Fatal(err)
		}
		report, _ := f.run(t)
		want := "not-run: at head: go.mod or go.sum is not a regular file gitleaks could read, so the go tool was not run on it"
		if got := targetOutcome(t, report, "go.mod"); got != want {
			t.Errorf("go.mod = %q, want %q", got, want)
		}
	})

	t.Run("a scanner that exits and leaves a child running", func(t *testing.T) {
		mark := filepath.Join(t.TempDir(), "child")
		f := prepare(t, seed{
			files: withSettings(map[string]string{"go.mod": "module example.com/app\n\ngo 1.26\n"}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "govulncheck", "sleep 300 >/dev/null 2>&1 &\necho $! > '"+mark+"'\n"+goStandIn())
			},
		})
		f.run(t)
		assertGone(t, readPID(t, mark))
	})

	t.Run("cgo code under a killed run", func(t *testing.T) {
		var reached time.Duration
		for _, signal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGABRT} {
			t.Run(signal.String(), func(t *testing.T) {
				mark := filepath.Join(t.TempDir(), "reached")
				reached = killedRun(t, cgoFixture(t, mark), mark, signal)
			})
		}

		t.Run("an exhausted budget", func(t *testing.T) {
			mark := filepath.Join(t.TempDir(), "reached")
			f := cgoFixture(t, mark)
			goTemp := t.TempDir()
			report, _ := f.run(t, budgetMarker+"="+(2*reached+3*time.Second).String(), "GOTMPDIR="+goTemp)
			assertSeededFound(t, report)
			if left := entriesUnder(t, goTemp); len(left) != 0 {
				t.Errorf("the run left files in GOTMPDIR: %v", left)
			}
			if _, err := os.Stat(mark); err != nil {
				t.Fatal("the budget ran out before go reached the cgo code, so nothing here proves it was stopped")
			}
			assertGone(t, readPID(t, mark))
			if got := targetOutcome(t, report, "bound/go.mod"); got != "not-run: at head: govulncheck on bound/go.mod ran out of time" {
				t.Errorf("bound/go.mod = %q", got)
			}
			if report.Sweep.Status != evidence.StatusRan {
				t.Errorf("sweep = %+v, want it to have run after the budget ran out", report.Sweep)
			}
		})
	})

	t.Run("a signal while gitleaks holds the value in its report", func(t *testing.T) {
		for _, pass := range []string{"added lines", "sweep"} {
			t.Run(pass, func(t *testing.T) {
				observed := t.TempDir()
				mark := filepath.Join(observed, "reached")
				mode := filepath.Join(observed, "mode")
				real := installed(t, "gitleaks")
				left := filepath.Join(observed, "left")
				body := `while [ $# -gt 0 ]; do
  if [ "$1" = --report-path ]; then
    shift
    if [ -e "$1" ]; then : > '` + left + `'; fi
    printf '%s' '[{"RuleID":"github-pat","Secret":"` + token + `"}]' > "$1"
    ls -ld "${1%/*}" > '` + mode + `'
  fi
  shift
done
echo $$ > '` + mark + `'
exec sleep 300
`
				f := prepare(t, seed{files: withSettings(map[string]string{}), ownGitleaks: true, scanners: func(stubs string) {
					script := `case "$*" in
version|*--redact*) exec '` + real + `' "$@" ;;
esac
` + body
					if pass == "sweep" {
						script = sweepPass(stubs, real, body)
					}
					writeStandIn(t, stubs, "gitleaks", script)
				}})
				killedRun(t, f, mark, syscall.SIGTERM)
				listing, err := os.ReadFile(mode)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.HasPrefix(listing, []byte("drwx------")) {
					t.Errorf("the value sat in a directory reading %s, want mode 0700", listing)
				}
				if _, err := os.Stat(left); err == nil {
					t.Error("the report of the pass before was still on disk")
				}
			})
		}
	})

	t.Run("a sweep that fails", func(t *testing.T) {
		real := installed(t, "gitleaks")
		for name, body := range map[string]string{
			"crashing":                        "exit 3\n",
			"named no value":                  sweepReport(`[{"RuleID":"github-pat"}]`),
			"named a value that is not there": sweepReport(`[{"RuleID":"github-pat","Secret":"decoded-from-something-else"}]`),
		} {
			t.Run(name, func(t *testing.T) {
				f := prepare(t, seed{
					ownGitleaks: true,
					files: withSettings(map[string]string{
						token + "/package.json": "{}\n",
						workflowFile:            usesWorkflow("actions/" + token + "@v1"),
					}),
					scanners: func(stubs string) {
						writeStandIn(t, stubs, "gitleaks", sweepPass(stubs, real, body))
					},
				})
				report, _ := f.run(t)
				if report.Sweep.Status != evidence.StatusNotRun || report.Sweep.Reason == "" {
					t.Errorf("sweep = %+v, want it to say it failed", report.Sweep)
				}
				for checkName, check := range map[string]evidence.Check{"secrets": report.Secrets, "workflows": report.Workflows} {
					if check.Status != evidence.StatusNotRun || !strings.Contains(check.Reason, "withheld") {
						t.Errorf("%s = %+v, want it withheld", checkName, check)
					}
					if len(check.New) != 0 {
						t.Errorf("%s listed %+v from a report nothing swept", checkName, check.New)
					}
				}
				if report.Dependencies.Status != evidence.StatusNotRun || len(report.Dependencies.Targets) != 0 {
					t.Errorf("dependencies = %+v, want no path from a report nothing swept", report.Dependencies)
				}
			})
		}
	})

	t.Run("a gitleaks that is gone after the added lines", func(t *testing.T) {
		real := installed(t, "gitleaks")
		remove, err := exec.LookPath("rm")
		if err != nil {
			t.Skip("rm is not on PATH")
		}
		ranScanner := filepath.Join(t.TempDir(), "ran")
		f := prepare(t, seed{
			noGitleaks: true,
			files: withSettings(map[string]string{
				"go.mod":     "module example.com/app\n\ngo 1.26\n",
				workflowFile: usesWorkflow("actions/" + token + "@v1"),
			}),
			scanners: func(stubs string) {
				writeStandIn(t, stubs, "govulncheck", ": > '"+ranScanner+"'\n"+goStandIn())
				writeStandIn(t, stubs, "gitleaks", `if [ "$1" = version ]; then exec '`+real+`' "$@"; fi
`+remove+` '`+filepath.Join(stubs, "gitleaks")+`'
exec '`+real+`' "$@"
`)
			},
		})
		report, _ := f.run(t)

		if _, err := os.Stat(ranScanner); err == nil {
			t.Error("the go tool ran although nothing could read its manifests")
		}
		if report.Sweep.Status != evidence.StatusNotRun || !strings.Contains(report.Sweep.Reason, "no sweep") {
			t.Errorf("sweep = %+v, want it to say there was none", report.Sweep)
		}
		if len(report.Workflows.New) != 0 || len(report.Dependencies.Targets) != 0 {
			t.Errorf("report = %+v, want nothing from a report nothing swept", report)
		}
	})

	t.Run("a sweep with the added lines unread", func(t *testing.T) {
		real := installed(t, "gitleaks")
		f := prepare(t, seed{
			ownGitleaks: true,
			files:       map[string]string{"notes.txt": "plain\n"},
			scanners: func(stubs string) {
				passed := filepath.Join(stubs, "added-lines-failed")
				writeStandIn(t, stubs, "gitleaks", `case "$*" in
version|*--redact*) exec '`+real+`' "$@" ;;
esac
if [ ! -e '`+passed+`' ]; then : > '`+passed+`'; exit 3; fi
exec '`+real+`' "$@"
`)
			},
		})
		report, _ := f.run(t)
		if report.Secrets.Status != evidence.StatusNotRun {
			t.Errorf("secrets = %+v, want not-run", report.Secrets)
		}
		if report.Sweep.Status != evidence.StatusRan || !strings.Contains(report.Sweep.Reason, "added lines could not be read") {
			t.Errorf("sweep = %+v, want it to say what it could not know", report.Sweep)
		}
	})

	t.Run("no secret leaves a report as it was", func(t *testing.T) {
		f := prepare(t, seed{
			files: map[string]string{
				"go.mod":     "module example.com/app\n\ngo 1.26\n",
				"Cargo.lock": "version = 3\n",
				workflowFile: "on: push\njobs:\n  ci:\n    steps:\n      - uses: actions/checkout@v4\n",
			},
			scanners: func(stubs string) {
				finding := `{"finding":{"osv":"GO-2026-0001","trace":[{"module":"example.com/dep","version":"v1.0.0","package":"example.com/dep/pkg","function":"Bad"}]}}`
				writeStandIn(t, stubs, "govulncheck", goStandIn(finding))
				writeStandIn(t, stubs, "osv-scanner", osvStandIn(`{"results":[{"packages":[{"package":{"name":"time","version":"0.1.43"},"groups":[{"ids":["RUSTSEC-2020-0071"],"aliases":["CVE-2020-26235"]}]}]}]}`))
			},
		})
		report, first := f.run(t)
		_, second := f.run(t)
		if !bytes.Equal(first, second) {
			t.Errorf("two runs differ:\n%s\n%s", first, second)
		}
		if bytes.Contains(first, []byte("REDACTED")) {
			t.Errorf("the sweep changed a report with no secret in it: %s", first)
		}
		if report.Sweep.Status != evidence.StatusRan || len(report.Dependencies.New) != 2 || len(report.Workflows.New) != 1 {
			t.Errorf("report = %s, want the findings kept and the sweep run", first)
		}
		if !strings.Contains(report.Sweep.Reason, "what gitleaks recognises") {
			t.Errorf("sweep = %+v, want it to say what it covers", report.Sweep)
		}
	})

	t.Run("two runs over a swept report", func(t *testing.T) {
		f := prepare(t, seed{files: withSettings(map[string]string{
			token + "/package.json": "{}\n",
			workflowFile:            "on: push\njobs:\n  ci:\n    steps:\n      - uses: actions/" + token + "@v1\n",
		})})
		_, first := f.run(t)
		_, second := f.run(t)
		if !bytes.Equal(first, second) {
			t.Errorf("two runs differ:\n%s\n%s", first, second)
		}
	})
}
