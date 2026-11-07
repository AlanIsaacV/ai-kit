package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

const toolName = "memory"

type callRecord struct {
	At            string `json:"at"`
	Tool          string `json:"tool"`
	Version       string `json:"version"`
	Session       string `json:"session"`
	Operation     string `json:"operation"`
	Interpreted   any    `json:"interpreted"`
	Outcome       string `json:"outcome"`
	OutcomeDetail string `json:"outcome_detail"`
	Count         *int   `json:"count"`
	Returned      *int   `json:"returned"`
	LatencyMs     int64  `json:"latency_ms"`
}

type call struct {
	start       time.Time
	operation   string
	echo        any
	interpreted any
	collection  bool
	count       int
	returned    int
	truncated   bool
	countOnly   bool
	outcome     string
	detail      string
}

var thisCall = &call{}

func (c *call) found(count, returned int, truncated, countOnly bool) {
	c.collection = true
	c.count, c.returned, c.truncated, c.countOnly = count, returned, truncated, countOnly
}

func (c *call) succeeded() {
	c.interpreted = c.echo
	switch {
	case !c.collection:
		c.outcome = "ok"
	case c.truncated && !c.countOnly:
		c.outcome = "truncated"
	case c.count == 0:
		c.outcome = "empty"
	default:
		c.outcome = "ok"
	}
}

var failureCodes = map[string]bool{
	"error":        true,
	"read_failed":  true,
	"write_failed": true,
	"init_failed":  true,
	"id_failed":    true,
}

func (c *call) refused(code string) {
	c.collection = false
	c.outcome, c.detail = "invalid", code
	if failureCodes[code] {
		c.outcome = "error"
	}
}

func (c *call) record() callRecord {
	r := callRecord{
		At:            c.start.UTC().Format(time.RFC3339),
		Tool:          toolName,
		Version:       buildVersion(),
		Session:       os.Getenv("CLAUDE_CODE_SESSION_ID"),
		Operation:     c.operation,
		Interpreted:   c.interpreted,
		Outcome:       c.outcome,
		OutcomeDetail: c.detail,
		LatencyMs:     time.Since(c.start).Milliseconds(),
	}
	if c.collection {
		count, returned := c.count, c.returned
		r.Count, r.Returned = &count, &returned
	}
	return r
}

func (c *call) write() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	line, err := json.Marshal(c.record())
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".ai-kit", "calls")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, toolName+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "unknown"
	}
	if dirty {
		return revision + "-dirty"
	}
	return revision
}
