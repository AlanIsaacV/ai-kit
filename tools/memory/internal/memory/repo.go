package memory

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

var repoNamePart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var scpGitHub = regexp.MustCompile(`^[A-Za-z0-9_.-]+@(?i:github\.com|ssh\.github\.com):(.+)$`)

type RepoLog struct {
	Path  string
	Scope Scope
	Names []string
}

func SplitRepoName(name string) (string, string, bool) {
	owner, repo, ok := strings.Cut(strings.TrimSuffix(name, ".git"), "/")
	if !ok {
		return "", "", false
	}
	if !repoNamePart.MatchString(owner) || !repoNamePart.MatchString(repo) ||
		owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", "", false
	}
	return owner, repo, true
}

func RepoName(repoID string) (string, bool) {
	raw := strings.TrimSpace(repoID)
	var path string
	if m := scpGitHub.FindStringSubmatch(raw); m != nil {
		path = m[1]
	} else {
		u, err := url.Parse(raw)
		if err != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", false
		}
		switch u.Scheme {
		case "https", "http", "ssh", "git":
		default:
			return "", false
		}
		if host := u.Hostname(); !strings.EqualFold(host, "github.com") && !strings.EqualFold(host, "ssh.github.com") {
			return "", false
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	owner, repo, ok := SplitRepoName(strings.TrimSuffix(path, "/"))
	if !ok {
		return "", false
	}
	return owner + "/" + repo, true
}

func RepoLogs(store string) ([]RepoLog, error) {
	path, err := (&Store{StorePath: store}).store()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	db, err := openStore(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT source_path, scope, repo_id FROM lines ORDER BY source_path ASC, line_number ASC`)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer rows.Close()

	type normalized struct {
		name string
		ok   bool
	}
	names := map[string]normalized{}
	var logs []RepoLog
	for rows.Next() {
		var source, scope string
		var repoID sql.NullString
		if err := rows.Scan(&source, &scope, &repoID); err != nil {
			return logs, fmt.Errorf("read %s: %w", path, err)
		}
		if len(logs) == 0 || logs[len(logs)-1].Path != source {
			logs = append(logs, RepoLog{Path: source, Scope: Scope(scope)})
		}
		n, seen := names[repoID.String]
		if !seen {
			n.name, n.ok = RepoName(repoID.String)
			names[repoID.String] = n
		}
		if !n.ok {
			continue
		}
		l := &logs[len(logs)-1]
		if !containsNameFold(l.Names, n.name) {
			l.Names = append(l.Names, n.name)
		}
	}
	if err := rows.Err(); err != nil {
		return logs, fmt.Errorf("read %s: %w", path, err)
	}
	return logs, nil
}

func LogsOfRepo(logs []RepoLog, name string) []string {
	var out []string
	for _, l := range logs {
		if l.Scope == ScopeRepo && containsNameFold(l.Names, name) {
			out = append(out, l.Path)
		}
	}
	return out
}

func UnattributedLogs(logs []RepoLog) []string {
	var out []string
	for _, l := range logs {
		if l.Scope == ScopeRepo && len(l.Names) == 0 {
			out = append(out, l.Path)
		}
	}
	return out
}

func SimilarRepoNames(logs []RepoLog, name string, n int) []string {
	want := strings.ToLower(name)
	reach := max(2, len(want)/4)
	type candidate struct {
		name     string
		distance int
	}
	seen := map[string]bool{}
	var candidates []candidate
	for _, l := range logs {
		if l.Scope != ScopeRepo {
			continue
		}
		for _, known := range l.Names {
			key := strings.ToLower(known)
			if seen[key] {
				continue
			}
			seen[key] = true
			if d := editDistance(want, key); d <= reach {
				candidates = append(candidates, candidate{known, d})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return strings.ToLower(candidates[i].name) < strings.ToLower(candidates[j].name)
	})
	out := []string{}
	for _, c := range candidates[:min(n, len(candidates))] {
		out = append(out, c.name)
	}
	return out
}

func containsNameFold(names []string, name string) bool {
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
