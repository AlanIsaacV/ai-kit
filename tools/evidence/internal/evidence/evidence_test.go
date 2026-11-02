package evidence

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	goConfigLine = `{"config":{"scanner_version":"%VERSION%","db":"https://vuln.go.dev","db_last_modified":"2026-10-01T00:00:00Z","go_version":"go1.27.3"}}`
	goCalledLine = `{"finding":{"osv":"%ID%","trace":[{"module":"example.com/dep","version":"v1.0.0","package":"example.com/dep/pkg","function":"Bad"},{"module":"example.com/app","package":"example.com/app","function":"main"}]}}`
	goModuleLine = `{"finding":{"osv":"GO-2026-0404","trace":[{"module":"example.com/other","version":"v1.0.0"}]}}`

	osvOneGroup = `{"results":[{"packages":[{"package":{"name":"time","version":"0.1.43"},"groups":[{"ids":["RUSTSEC-2020-0071"],"aliases":["CVE-2020-26235"]}]}]}]}`
	osvTwoGroup = `{"results":[{"packages":[{"package":{"name":"time","version":"0.1.43"},"groups":[{"ids":["RUSTSEC-2020-0071"],"aliases":["CVE-2020-26235"]}]},{"package":{"name":"smallvec","version":"1.0.0"},"groups":[{"ids":["RUSTSEC-2021-0003"],"aliases":["CVE-2021-25900"]}]}]}]}`
)

func writeTree(t *testing.T, root string, files map[string]string) {
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

func standIn(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func gitleaksStandIn(version, added, others string, exitCode string) string {
	return `if [ "$1" = version ]; then echo ` + version + `; exit 0; fi
IFS= read -r lead
IFS= read -r input
while [ $# -gt 0 ]; do
  if [ "$1" = --report-path ]; then
    shift
    case "$input" in
    two*) printf '%s' '` + added + `' > "$1" ;;
    *) printf '%s' '` + others + `' > "$1" ;;
    esac
  fi
  shift
done
exit ` + exitCode + "\n"
}

func goStandIn(stream, exitCode string) string {
	return "printf '%s\\n' '" + stream + "'\nexit " + exitCode + "\n"
}

func osvStandIn(version, output, exitCode string) string {
	return `if [ "$1" = --version ]; then echo 'osv-scanner version: ` + version + `'; exit 0; fi
printf '%s' '` + output + `'
exit ` + exitCode + "\n"
}

func blindGitleaks() string {
	return gitleaksStandIn(gitleaksPin, "[]", "[]", "0")
}

func isolate(t *testing.T) string {
	t.Helper()
	stubs := t.TempDir()
	t.Setenv("PATH", stubs)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	return stubs
}

func runFixture(t *testing.T, headFiles, baseFiles map[string]string, diff string) Report {
	t.Helper()
	root := t.TempDir()
	head := filepath.Join(root, "head")
	base := filepath.Join(root, "base")
	writeTree(t, head, headFiles)
	writeTree(t, base, baseFiles)
	diffPath := filepath.Join(root, "change.diff")
	if err := os.WriteFile(diffPath, []byte(diff), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), Options{Head: head, Base: base, Diff: diffPath})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func assertNotRun(t *testing.T, name string, check Check, reasonPart string) {
	t.Helper()
	if check.Status != StatusNotRun {
		t.Errorf("%s: status = %q, want %q", name, check.Status, StatusNotRun)
	}
	if !strings.Contains(check.Reason, reasonPart) {
		t.Errorf("%s: reason = %q, want it to contain %q", name, check.Reason, reasonPart)
	}
	if len(check.New) != 0 {
		t.Errorf("%s: listed findings %+v although it did not run", name, check.New)
	}
	if check.Inherited != nil {
		t.Errorf("%s: counted %d inherited although it did not run", name, *check.Inherited)
	}
}

func TestScannerThatDidNotRunNeverReadsAsClean(t *testing.T) {
	tree := map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.26\n",
		"Cargo.lock": "version = 3\n",
		"src.txt":    "one\ntwo\n",
	}
	diff := "diff --git a/src.txt b/src.txt\n--- a/src.txt\n+++ b/src.txt\n@@ -1 +1,2 @@\n one\n+two\n"
	leak := `[{"RuleID":"github-pat","StartLine":2}]`
	called := strings.ReplaceAll(goCalledLine, "%ID%", "GO-2026-0001")

	t.Run("gitleaks absent", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, govulncheckName, goStandIn(strings.ReplaceAll(goConfigLine, "%VERSION%", govulncheckPin)+"\n"+called, "0"))
		standIn(t, stubs, osvScannerName, osvStandIn(osvScannerPin, osvOneGroup, "1"))
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "secrets", report.Secrets, "not installed")
		assertNotRun(t, "dependencies", report.Dependencies, "no sweep")
		assertNotRun(t, "workflows", report.Workflows, "no sweep")
		if len(report.Dependencies.Targets) != 0 {
			t.Errorf("targets = %+v, want none from a run with no sweep", report.Dependencies.Targets)
		}
		if report.Sweep.Status != StatusNotRun || !strings.Contains(report.Sweep.Reason, "no sweep") {
			t.Errorf("sweep = %+v, want it to say there is none", report.Sweep)
		}
	})

	t.Run("absent", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, blindGitleaks())
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "dependencies", report.Dependencies, "could be scanned")
		if len(report.Dependencies.Targets) != 2 {
			t.Errorf("targets = %+v, want both named", report.Dependencies.Targets)
		}
		for _, target := range report.Dependencies.Targets {
			if target.Status != StatusNotRun || !strings.Contains(target.Reason, "not installed") {
				t.Errorf("target %+v should be not-run because its scanner is absent", target)
			}
		}
	})

	t.Run("crashing", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, gitleaksStandIn(gitleaksPin, "[]", "[]", "0"))
		standIn(t, stubs, govulncheckName, goStandIn(strings.ReplaceAll(goConfigLine, "%VERSION%", govulncheckPin)+"\n"+called, "3"))
		standIn(t, stubs, osvScannerName, osvStandIn(osvScannerPin, osvOneGroup, "3"))
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "dependencies", report.Dependencies, "could be scanned")
		if len(report.Dependencies.Targets) != 2 {
			t.Errorf("targets = %+v, want both named", report.Dependencies.Targets)
		}
		for _, target := range report.Dependencies.Targets {
			if target.Status != StatusNotRun || !strings.Contains(target.Reason, "exited with status 3") {
				t.Errorf("target %+v should be not-run because its scanner crashed", target)
			}
		}
		if report.Sweep.Status != StatusRan {
			t.Errorf("sweep = %+v, want it to have run", report.Sweep)
		}
	})

	t.Run("gitleaks crashing", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, gitleaksStandIn(gitleaksPin, leak, "[]", "3"))
		standIn(t, stubs, govulncheckName, goStandIn(strings.ReplaceAll(goConfigLine, "%VERSION%", govulncheckPin)+"\n"+called, "0"))
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "secrets", report.Secrets, "exited with status 3")
		assertNotRun(t, "dependencies", report.Dependencies, "could be scanned")
		if report.Sweep.Status != StatusNotRun || !strings.Contains(report.Sweep.Reason, "exited with status 3") {
			t.Errorf("sweep = %+v, want it to say gitleaks crashed", report.Sweep)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, gitleaksStandIn(gitleaksPin, "[]", "[]", "0"))
		standIn(t, stubs, govulncheckName, goStandIn("no json here", "0"))
		standIn(t, stubs, osvScannerName, osvStandIn(osvScannerPin, "no json here", "0"))
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "dependencies", report.Dependencies, "could be scanned")
		if len(report.Dependencies.Targets) != 2 {
			t.Errorf("targets = %+v, want both named", report.Dependencies.Targets)
		}
		for _, target := range report.Dependencies.Targets {
			if target.Status != StatusNotRun || !strings.Contains(target.Reason, "unreadable") {
				t.Errorf("target %+v should be not-run because its scanner printed garbage", target)
			}
		}
	})

	t.Run("gitleaks unreadable", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, gitleaksStandIn(gitleaksPin, "no json here", "no json here", "0"))
		report := runFixture(t, tree, tree, diff)

		assertNotRun(t, "secrets", report.Secrets, "unreadable")
		if report.Sweep.Status != StatusNotRun || !strings.Contains(report.Sweep.Reason, "unreadable") {
			t.Errorf("sweep = %+v, want it to say gitleaks printed garbage", report.Sweep)
		}
	})

	t.Run("mismatched", func(t *testing.T) {
		stubs := isolate(t)
		standIn(t, stubs, gitleaksName, gitleaksStandIn("9.9.9", leak, "[]", "0"))
		standIn(t, stubs, govulncheckName, goStandIn(strings.ReplaceAll(goConfigLine, "%VERSION%", "v9.9.9")+"\n"+called, "0"))
		standIn(t, stubs, osvScannerName, osvStandIn("9.9.9", osvOneGroup, "1"))
		report := runFixture(t, tree, map[string]string{"src.txt": "one\n"}, diff)

		if report.Secrets.Status != StatusRan || len(report.Secrets.New) != 1 {
			t.Errorf("secrets should run on a mismatched version: %+v", report.Secrets)
		}
		if report.Dependencies.Status != StatusRan || len(report.Dependencies.New) != 2 {
			t.Errorf("dependencies should run on a mismatched version: %+v", report.Dependencies)
		}
		wantPins := map[string]string{gitleaksName: gitleaksPin, govulncheckName: govulncheckPin, osvScannerName: osvScannerPin}
		var scanners []Scanner
		scanners = append(scanners, report.Secrets.Scanners...)
		scanners = append(scanners, report.Dependencies.Scanners...)
		if len(scanners) != len(wantPins) {
			t.Fatalf("scanners = %+v, want one for each of %v", scanners, wantPins)
		}
		for _, scanner := range scanners {
			if scanner.Pinned != wantPins[scanner.Name] || !strings.HasSuffix(scanner.Version, "9.9.9") || scanner.AtPin {
				t.Errorf("scanner %+v should name both its version and the pinned one", scanner)
			}
		}
	})
}

func TestInheritedFindingIsNotCountedAsNew(t *testing.T) {
	stubs := isolate(t)
	standIn(t, stubs, gitleaksName, blindGitleaks())
	config := strings.ReplaceAll(goConfigLine, "%VERSION%", govulncheckPin)
	shared := strings.ReplaceAll(goCalledLine, "%ID%", "GO-2026-0001")
	introduced := strings.ReplaceAll(goCalledLine, "%ID%", "GO-2026-0002")
	standIn(t, stubs, govulncheckName, `case "$PWD" in
*/base*) printf '%s\n' '`+config+`' '`+shared+`' ;;
*) printf '%s\n' '`+config+`' '`+shared+`' '`+introduced+`' '`+goModuleLine+`' ;;
esac
`)
	standIn(t, stubs, osvScannerName, `if [ "$1" = --version ]; then echo 'osv-scanner version: `+osvScannerPin+`'; exit 0; fi
case "$*" in
*/base/*) printf '%s' '`+osvOneGroup+`' ;;
*) printf '%s' '`+osvTwoGroup+`' ;;
esac
exit 1
`)

	baseWorkflow := "on: push\njobs:\n  ci:\n    steps:\n      - uses: actions/checkout@v3\n      - run: make\n"
	headWorkflow := "on: push\njobs:\n  ci:\n    steps:\n      - run: make\n      - uses: actions/checkout@v3\n      - uses: actions/setup-go@v5\n"
	base := map[string]string{
		"go.mod":                   "module example.com/app\n\ngo 1.26\n",
		"Cargo.lock":               "version = 3\n",
		".github/workflows/ci.yml": baseWorkflow,
	}
	head := map[string]string{
		"go.mod":                   "module example.com/app\n\ngo 1.26\n",
		"Cargo.lock":               "version = 3\n",
		".github/workflows/ci.yml": headWorkflow,
	}
	diff := strings.Join([]string{
		"diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml",
		"--- a/.github/workflows/ci.yml",
		"+++ b/.github/workflows/ci.yml",
		"@@ -2,5 +2,6 @@",
		" jobs:",
		"   ci:",
		"     steps:",
		"-      - uses: actions/checkout@v3",
		"       - run: make",
		"+      - uses: actions/checkout@v3",
		"+      - uses: actions/setup-go@v5",
		"",
	}, "\n")

	report := runFixture(t, head, base, diff)

	var newDependencies []string
	for _, finding := range report.Dependencies.New {
		newDependencies = append(newDependencies, finding.Path+" "+finding.Rule)
	}
	wantDependencies := []string{"Cargo.lock RUSTSEC-2021-0003", "go.mod GO-2026-0002"}
	if !slices.Equal(newDependencies, wantDependencies) {
		t.Errorf("new dependency findings = %v, want %v", newDependencies, wantDependencies)
	}
	if report.Dependencies.Inherited == nil || *report.Dependencies.Inherited != 2 {
		t.Errorf("inherited dependency findings = %v, want 2", report.Dependencies.Inherited)
	}

	if len(report.Workflows.New) != 1 {
		t.Fatalf("new workflow findings = %+v, want only the action the change added", report.Workflows.New)
	}
	got := report.Workflows.New[0]
	if got.Path != ".github/workflows/ci.yml" || got.Line != 7 || got.Rule != ruleUnpinned || got.Detail != "actions/setup-go@v5" {
		t.Errorf("new workflow finding = %+v", got)
	}
	if report.Workflows.Inherited == nil || *report.Workflows.Inherited != 1 {
		t.Errorf("inherited workflow findings = %v, want 1", report.Workflows.Inherited)
	}
}
