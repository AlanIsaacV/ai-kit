package measure

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

const (
	attributionFile = "attribution.jsonl"
	attributionCap  = 15
	keyWidth        = 120
)

type Attribution struct {
	Probe  string   `json:"probe"`
	Source string   `json:"source"`
	Unit   string   `json:"unit"`
	Base   *Ranking `json:"base"`
	Head   *Ranking `json:"head"`
}

type Ranking struct {
	Runs    int     `json:"runs"`
	Entries []Share `json:"entries"`
}

type Share struct {
	Key   string `json:"key"`
	Share Stats  `json:"share"`
	Value Stats  `json:"value"`
}

type attributionGroup struct{ probe, source, unit string }

type runAttribution map[attributionGroup]map[string]float64

func parseAttribution(path string) (runAttribution, int, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	defer file.Close()
	values := runAttribution{}
	ignored := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		group, key, value, ok := parseAttributionLine(line)
		if !ok {
			ignored++
			continue
		}
		if values[group] == nil {
			values[group] = map[string]float64{}
		}
		values[group][key] += value
	}
	return values, ignored, true, scanner.Err()
}

func parseAttributionLine(line []byte) (attributionGroup, string, float64, bool) {
	var raw struct {
		Probe  *string  `json:"probe"`
		Source *string  `json:"source"`
		Key    *string  `json:"key"`
		Value  *float64 `json:"value"`
		Unit   *string  `json:"unit"`
	}
	if line[0] != '{' || json.Unmarshal(line, &raw) != nil {
		return attributionGroup{}, "", 0, false
	}
	if raw.Probe == nil || *raw.Probe == "" || raw.Source == nil || *raw.Source == "" || raw.Key == nil || *raw.Key == "" || raw.Unit == nil || raw.Value == nil || *raw.Value < 0 {
		return attributionGroup{}, "", 0, false
	}
	return attributionGroup{probe: *raw.Probe, source: *raw.Source, unit: *raw.Unit}, *raw.Key, *raw.Value, true
}

func buildAttribution(runs []runResult) []Attribution {
	perSide := map[attributionGroup]map[string][]map[string]float64{}
	for _, run := range runs {
		if run.failed {
			continue
		}
		for group, values := range run.attribution {
			total := 0.0
			for _, v := range values {
				total += v
			}
			if total == 0 {
				continue
			}
			if perSide[group] == nil {
				perSide[group] = map[string][]map[string]float64{}
			}
			perSide[group][run.side] = append(perSide[group][run.side], values)
		}
	}
	groups := make([]attributionGroup, 0, len(perSide))
	for group := range perSide {
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b attributionGroup) int {
		if c := strings.Compare(a.probe, b.probe); c != 0 {
			return c
		}
		if c := strings.Compare(a.source, b.source); c != 0 {
			return c
		}
		return strings.Compare(a.unit, b.unit)
	})
	out := make([]Attribution, 0, len(groups))
	for _, group := range groups {
		out = append(out, Attribution{
			Probe:  group.probe,
			Source: group.source,
			Unit:   group.unit,
			Base:   rank(perSide[group][SideBase]),
			Head:   rank(perSide[group][SideHead]),
		})
	}
	return out
}

func rank(runs []map[string]float64) *Ranking {
	if len(runs) == 0 {
		return nil
	}
	totals := make([]float64, len(runs))
	keys := map[string]bool{}
	for i, values := range runs {
		for key, v := range values {
			totals[i] += v
			keys[key] = true
		}
	}
	entries := make([]Share, 0, len(keys))
	for key := range keys {
		shares := make([]float64, len(runs))
		values := make([]float64, len(runs))
		for i, run := range runs {
			values[i] = run[key]
			shares[i] = run[key] / totals[i]
		}
		entries = append(entries, Share{Key: key, Share: *stats(shares), Value: *stats(values)})
	}
	slices.SortFunc(entries, func(a, b Share) int {
		if a.Share.Median != b.Share.Median {
			if a.Share.Median > b.Share.Median {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Key, b.Key)
	})
	return &Ranking{Runs: len(runs), Entries: entries}
}

func encodeAttribution(w io.Writer, r Report) error {
	if r.AttributionIgnored == nil && len(r.Attribution) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("\nattribution  share of each probe's time per key, median over the side's runs that wrote it\n")
	if r.AttributionIgnored != nil {
		fmt.Fprintf(&b, "ignored      %s lines that are not attribution: base %d, head %d\n", attributionFile, r.AttributionIgnored.Base, r.AttributionIgnored.Head)
	}
	if len(r.Attribution) == 0 {
		b.WriteString("\nno attribution from any completed run\n")
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	for _, a := range r.Attribution {
		if _, err := fmt.Fprintf(w, "\n%s  %s  %s\n", a.Probe, a.Source, a.Unit); err != nil {
			return err
		}
		for _, side := range []string{SideBase, SideHead} {
			ranking := a.Base
			if side == SideHead {
				ranking = a.Head
			}
			if err := encodeRanking(w, side, a.Unit, ranking); err != nil {
				return err
			}
		}
	}
	return nil
}

func encodeRanking(w io.Writer, side, unit string, ranking *Ranking) error {
	if ranking == nil {
		_, err := fmt.Fprintf(w, "  %s  no attribution from any completed run\n", side)
		return err
	}
	if _, err := fmt.Fprintf(w, "  %s  over %d runs, %d keys\n", side, ranking.Runs, len(ranking.Entries)); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(table, "    SHARE\tMIN..MAX\tMEDIAN %s\tKEY\n", unitLabel(unit))
	shown := ranking.Entries
	if len(shown) > attributionCap {
		shown = shown[:attributionCap]
	}
	for _, e := range shown {
		fmt.Fprintf(table, "    %s\t%s\t%s\t%s\n",
			share(e.Share.Median), shareSpan(e.Share), number(e.Value.Median), shortKey(e.Key))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if cut := len(ranking.Entries) - len(shown); cut > 0 {
		_, err := fmt.Fprintf(w, "    %d more cut; the full ranking is in --json\n", cut)
		return err
	}
	return nil
}

func unitLabel(unit string) string {
	if unit == "" {
		return "VALUE"
	}
	return unit
}

func share(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}

func shareSpan(s Stats) string {
	return fmt.Sprintf("%.1f..%.1f%%", s.Min*100, s.Max*100)
}

func shortKey(key string) string {
	key = strings.Join(strings.Fields(key), " ")
	if utf8.RuneCountInString(key) <= keyWidth {
		return key
	}
	runes := []rune(key)
	return string(runes[:keyWidth-3]) + "..."
}
