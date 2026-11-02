package evidence

import (
	"bytes"
	"cmp"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	ecosystemGo         = "go"
	ecosystemRust       = "rust"
	ecosystemPython     = "python"
	ecosystemJavaScript = "javascript"

	reasonJavaScript = "advisories for JavaScript are outside this cut"
	reasonNoCargo    = "a Cargo.toml with no Cargo.lock states no resolved version"
	reasonNested     = "the image installs the root uv.lock, and this one is not it"
	reasonUvNoLock   = "the root Dockerfile runs uv sync but there is no root uv.lock"
	reasonPip        = "the root Dockerfile installs with pip, so no lockfile states what is deployed"
	reasonNoUvSync   = "the root Dockerfile does not run uv sync, so no lockfile states what is deployed"
	reasonNoDocker   = "there is no root Dockerfile, so no lockfile states what is deployed"
)

var (
	skippedDirs = map[string]bool{
		"vendor":           true,
		"vendors":          true,
		"node_modules":     true,
		"bower_components": true,
		"third_party":      true,
		"third-party":      true,
		"testdata":         true,
		"__pycache__":      true,
	}
	pythonManifests = regexp.MustCompile(`^(?:pyproject\.toml|Pipfile|Pipfile\.lock|poetry\.lock|setup\.py|requirements[^/]*\.txt)$`)
	uvSync          = regexp.MustCompile(`\buv\s+(?:-\S+\s+)*sync\b`)
	pipInstall      = regexp.MustCompile(`\bpip3?\s+install\b`)
)

type target struct {
	ecosystem string
	path      string
	reason    string
}

func detectTargets(root string) ([]target, error) {
	var found []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if current != root && (strings.HasPrefix(name, ".") || skippedDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}

	var targets []target
	var cargoLocks, cargoManifests, uvLocks []string
	otherPython := false
	for _, file := range found {
		switch name := path.Base(file); {
		case name == "go.mod":
			targets = append(targets, target{ecosystem: ecosystemGo, path: file})
		case name == "Cargo.lock":
			cargoLocks = append(cargoLocks, file)
			targets = append(targets, target{ecosystem: ecosystemRust, path: file})
		case name == "Cargo.toml":
			cargoManifests = append(cargoManifests, file)
		case name == "uv.lock":
			uvLocks = append(uvLocks, file)
		case name == "package.json":
			targets = append(targets, target{ecosystem: ecosystemJavaScript, path: file, reason: reasonJavaScript})
		case pythonManifests.MatchString(name):
			otherPython = true
		}
	}

	targets = append(targets, unlockedCargo(cargoManifests, cargoLocks)...)
	targets = append(targets, pythonTargets(root, uvLocks, otherPython)...)
	slices.SortFunc(targets, func(a, b target) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.ecosystem, b.ecosystem))
	})
	return targets, nil
}

func unlockedCargo(manifests, locks []string) []target {
	covered := map[string]bool{}
	for _, lock := range locks {
		covered[path.Dir(lock)] = true
	}
	var targets []target
	for _, manifest := range manifests {
		dir := path.Dir(manifest)
		locked := false
		for {
			if covered[dir] {
				locked = true
				break
			}
			if dir == "." {
				break
			}
			dir = path.Dir(dir)
		}
		if locked {
			continue
		}
		covered[path.Dir(manifest)] = true
		targets = append(targets, target{ecosystem: ecosystemRust, path: manifest, reason: reasonNoCargo})
	}
	return targets
}

func pythonTargets(root string, uvLocks []string, otherPython bool) []target {
	if len(uvLocks) == 0 && !otherPython {
		return nil
	}
	reason := deploymentReason(root, slices.Contains(uvLocks, "uv.lock"))
	if len(uvLocks) == 0 {
		return []target{{ecosystem: ecosystemPython, path: ".", reason: reason}}
	}
	var targets []target
	for _, lock := range uvLocks {
		entry := target{ecosystem: ecosystemPython, path: lock, reason: reason}
		if lock != "uv.lock" && reason == "" {
			entry.reason = reasonNested
		}
		targets = append(targets, entry)
	}
	return targets
}

func deploymentReason(root string, rootLock bool) string {
	raw, ok := readRegular(filepath.Join(root, "Dockerfile"))
	if !ok {
		return reasonNoDocker
	}
	var instructions bytes.Buffer
	for line := range bytes.SplitSeq(raw, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("#")) {
			continue
		}
		instructions.Write(line)
		instructions.WriteByte('\n')
	}
	switch {
	case uvSync.Match(instructions.Bytes()):
		if rootLock {
			return ""
		}
		return reasonUvNoLock
	case pipInstall.Match(instructions.Bytes()):
		return reasonPip
	}
	return reasonNoUvSync
}

func readRegular(path string) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return raw, true
}
