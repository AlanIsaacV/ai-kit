package evidence

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	reasonGoLoad     = "packages could not be loaded, which is often a private module"
	reasonGoEmpty    = "the module holds no package to scan"
	reasonGoSecret   = "go.mod or go.sum holds what gitleaks reads as a secret, so the go tool was not run on it"
	reasonGoManifest = "go.mod or go.sum is not a regular file gitleaks could read, so the go tool was not run on it"

	goLoadMarker = "loading packages"
	goNoPackages = 2
	goCacheDir   = "go-build"
	goSumFile    = "go.sum"
	goSumNames   = 2
)

var (
	goManifests = []string{"go.mod", goSumFile}
	goVersion   = regexp.MustCompile(`^[1-9][0-9]*\.(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*))?(?:[a-z]+[0-9]+)?$`)
)

var goEnvironment = []string{
	"GOTOOLCHAIN=local",
	"GOWORK=off",
	"GOFLAGS=-buildvcs=false",
}

type govulncheckMessage struct {
	Config  *govulncheckConfig  `json:"config"`
	OSV     *govulncheckOSV     `json:"osv"`
	Finding *govulncheckFinding `json:"finding"`
}

type govulncheckConfig struct {
	ScannerVersion string `json:"scanner_version"`
	DB             string `json:"db"`
	DBLastModified string `json:"db_last_modified"`
	GoVersion      string `json:"go_version"`
}

type govulncheckOSV struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases"`
}

type govulncheckFinding struct {
	OSV   string             `json:"osv"`
	Trace []govulncheckFrame `json:"trace"`
}

type govulncheckFrame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Package  string `json:"package"`
	Function string `json:"function"`
	Receiver string `json:"receiver"`
}

func (s *session) scanGoModule(ctx context.Context, tree string, t target) (scan, error) {
	binary, err := lookup(govulncheckName)
	if err != nil {
		return scan{}, err
	}
	dir := filepath.Join(tree, filepath.FromSlash(path.Dir(t.path)))
	if err := s.guardGoManifests(ctx, dir); err != nil {
		return scan{}, err
	}
	env := append(slices.Clone(goEnvironment), "GOCACHE="+filepath.Join(s.work, goCacheDir))
	result := s.execute(ctx, dir, env, nil, binary, "-scan", "symbol", "-format", "json", "./...")
	if result.err != nil {
		return scan{}, errors.New(goFailure(result, govulncheckName+" on "+t.path))
	}
	return parseGovulncheck(result.stdout)
}

func (s *session) guardGoManifests(ctx context.Context, dir string) error {
	gitleaks, err := lookup(gitleaksName)
	if err != nil {
		return err
	}
	var stream strings.Builder
	for _, name := range goManifests {
		full := filepath.Join(dir, name)
		raw, ok := readRegular(full)
		if ok {
			if name == goSumFile {
				raw = withoutHashes(raw)
			}
			stream.Write(raw)
			stream.WriteByte('\n')
			continue
		}
		if _, err := os.Lstat(full); !errors.Is(err, os.ErrNotExist) {
			return errors.New(reasonGoManifest)
		}
	}
	manifests := stream.String()
	if slices.ContainsFunc(s.values, func(value string) bool { return strings.Contains(manifests, value) }) {
		return errors.New(reasonGoSecret)
	}
	leaks, err := s.readLines(ctx, gitleaks, recognisableLines(manifests), 0, true)
	if err != nil {
		return err
	}
	if len(leaks) > 0 {
		return errors.New(reasonGoSecret)
	}
	return nil
}

func withoutHashes(sum []byte) []byte {
	var names bytes.Buffer
	for line := range bytes.SplitSeq(sum, []byte("\n")) {
		fields := bytes.Fields(line)
		names.Write(bytes.Join(fields[:min(len(fields), goSumNames)], []byte(" ")))
		names.WriteByte('\n')
	}
	return names.Bytes()
}

func goFailure(result execution, name string) string {
	reason := result.failure(name)
	switch {
	case bytes.Contains(bytes.ToLower(result.stderr), []byte(goLoadMarker)):
		return reason + ": " + reasonGoLoad
	case result.exitCode == goNoPackages:
		return reason + ": " + reasonGoEmpty
	}
	return reason
}

func parseGovulncheck(stdout []byte) (scan, error) {
	unreadable := errors.New(govulncheckName + " printed something unreadable")

	var config *govulncheckConfig
	aliases := map[string][]string{}
	called := map[string]*advisory{}
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var message govulncheckMessage
		err := decoder.Decode(&message)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return scan{}, unreadable
		}
		if message.Config != nil {
			config = message.Config
		}
		if message.OSV != nil {
			aliases[message.OSV.ID] = message.OSV.Aliases
		}
		if message.Finding == nil || len(message.Finding.Trace) == 0 || message.Finding.Trace[0].Function == "" {
			continue
		}
		frame := message.Finding.Trace[0]
		key := frame.Module + "\x00" + message.Finding.OSV
		entry, ok := called[key]
		if !ok {
			entry = &advisory{
				id:      message.Finding.OSV,
				ids:     []string{message.Finding.OSV},
				pkg:     frame.Module,
				version: frame.Version,
			}
			called[key] = entry
		}
		if symbol := frame.symbol(); !slices.Contains(entry.functions, symbol) {
			entry.functions = append(entry.functions, symbol)
		}
	}
	if config == nil || config.ScannerVersion == "" {
		return scan{}, unreadable
	}

	result := scan{
		scanner:   newScanner(govulncheckName, config.ScannerVersion, govulncheckPin, config.DB, config.DBLastModified),
		toolchain: config.GoVersion,
	}
	for _, key := range slices.Sorted(maps.Keys(called)) {
		entry := called[key]
		slices.Sort(entry.functions)
		entry.aliases = sortedCopy(aliases[entry.id])
		result.advisories = append(result.advisories, *entry)
	}
	slices.SortFunc(result.advisories, func(a, b advisory) int {
		return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.id, b.id))
	})
	return result, nil
}

func (f govulncheckFrame) symbol() string {
	name := f.Function
	if f.Receiver != "" {
		name = strings.TrimPrefix(f.Receiver, "*") + "." + name
	}
	return f.Package + "." + name
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := slices.Clone(values)
	slices.Sort(copied)
	return copied
}

func goModVersion(path string) string {
	raw, ok := readRegular(path)
	if !ok {
		return ""
	}
	inBlock := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		code, _, _ := strings.Cut(line, "//")
		fields := strings.Fields(code)
		switch {
		case len(fields) == 0:
		case inBlock:
			inBlock = !strings.HasPrefix(fields[0], ")")
		case strings.HasSuffix(fields[len(fields)-1], "("):
			inBlock = true
		case len(fields) == 2 && fields[0] == "go" && goVersion.MatchString(fields[1]):
			return fields[1]
		}
	}
	return ""
}
