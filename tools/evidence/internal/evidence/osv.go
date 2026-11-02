package evidence

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"
)

const (
	dateUnknown = "unknown"
	dateLayout  = "2006-01-02T15:04:05Z"

	osvDatabaseUnknown = "offline database"

	reasonOSVRead  = "the lockfile could not be read"
	reasonOSVEmpty = "the lockfile lists no package"

	osvExtractionMarker = "extraction failed"
	osvNoPackages       = 128
)

var (
	osvScannerVersion = regexp.MustCompile(`(?m)^osv-scanner version:\s*(\S+)`)
	osvDatabaseLine   = regexp.MustCompile(`(?m)^Loaded (\S+) local db from (.+)$`)
	osvDatabaseNames  = map[string]string{ecosystemRust: "crates.io", ecosystemPython: "PyPI"}
)

type osvOutput struct {
	Results *[]osvResult `json:"results"`
}

type osvResult struct {
	Packages []osvPackage `json:"packages"`
}

type osvPackage struct {
	Package struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"package"`
	Groups []struct {
		IDs     []string `json:"ids"`
		Aliases []string `json:"aliases"`
	} `json:"groups"`
}

func (s *session) scanLockfile(ctx context.Context, tree string, t target) (scan, error) {
	binary, err := lookup(osvScannerName)
	if err != nil {
		return scan{}, err
	}
	version, err := s.scannerVersion(ctx, osvScannerName, binary, osvScannerVersion, "--version")
	if err != nil {
		return scan{}, err
	}
	config := filepath.Join(s.work, "osv-scanner.toml")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		return scan{}, errors.New("the run could not write its own scanner settings")
	}
	result := s.execute(ctx, s.work, nil, nil, binary,
		"scan", "source",
		"--offline-vulnerabilities",
		"--download-offline-databases",
		"--no-call-analysis=all",
		"--no-resolve",
		"--config", config,
		"--format", "json",
		"-L", filepath.Join(tree, filepath.FromSlash(t.path)),
	)
	if result.exitCode != 0 && result.exitCode != 1 {
		return scan{}, errors.New(osvFailure(result, osvScannerName+" on "+t.path))
	}
	advisories, err := parseOSV(result.stdout)
	if err != nil {
		return scan{}, err
	}
	database, date := osvDatabase(result.stderr, t.ecosystem)
	return scan{
		scanner:    newScanner(osvScannerName, version, osvScannerPin, database, date),
		advisories: advisories,
	}, nil
}

func osvFailure(result execution, name string) string {
	reason := result.failure(name)
	switch {
	case bytes.Contains(result.stderr, []byte(osvExtractionMarker)):
		return reason + ": " + reasonOSVRead
	case result.exitCode == osvNoPackages:
		return reason + ": " + reasonOSVEmpty
	}
	return reason
}

func parseOSV(stdout []byte) ([]advisory, error) {
	var output osvOutput
	if err := json.Unmarshal(stdout, &output); err != nil || output.Results == nil {
		return nil, errors.New(osvScannerName + " printed something unreadable")
	}
	var advisories []advisory
	for _, result := range *output.Results {
		for _, pkg := range result.Packages {
			for _, group := range pkg.Groups {
				ids := sortedCopy(group.IDs)
				if len(ids) == 0 {
					continue
				}
				aliases := []string{}
				for _, alias := range sortedCopy(append(slices.Clone(group.IDs), group.Aliases...)) {
					if alias != ids[0] && !slices.Contains(aliases, alias) {
						aliases = append(aliases, alias)
					}
				}
				advisories = append(advisories, advisory{
					id:      ids[0],
					ids:     append([]string{ids[0]}, aliases...),
					aliases: aliases,
					pkg:     pkg.Package.Name,
					version: pkg.Package.Version,
				})
			}
		}
	}
	slices.SortFunc(advisories, func(a, b advisory) int {
		return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.version, b.version), cmp.Compare(a.id, b.id))
	})
	return advisories, nil
}

func osvDatabase(stderr []byte, ecosystem string) (string, string) {
	name, known := osvDatabaseNames[ecosystem]
	match := osvDatabaseLine.FindSubmatch(stderr)
	if !known || match == nil || string(match[1]) != name {
		return osvDatabaseUnknown, dateUnknown
	}
	info, err := os.Stat(string(match[2]))
	if err != nil {
		return name, dateUnknown
	}
	return name, info.ModTime().UTC().Truncate(time.Second).Format(dateLayout)
}
