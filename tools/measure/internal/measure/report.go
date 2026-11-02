package measure

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

type Report struct {
	Repo       string      `json:"repo"`
	Recipe     string      `json:"recipe"`
	Base       Ref         `json:"base"`
	Head       Ref         `json:"head"`
	Runs       int         `json:"runs"`
	Results    string      `json:"results"`
	Failed     SideCounts  `json:"failed"`
	Ignored    SideCounts  `json:"ignored_lines"`
	FailedRuns []FailedRun `json:"failed_runs"`
	Metrics    []Metric    `json:"metrics"`
}

type Ref struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

type SideCounts struct {
	Base int `json:"base"`
	Head int `json:"head"`
}

type FailedRun struct {
	Side       string   `json:"side"`
	Run        int      `json:"run"`
	ExitCode   int      `json:"exit_code"`
	Stderr     string   `json:"stderr"`
	StderrTail []string `json:"stderr_tail"`
}

type Metric struct {
	Probe        string   `json:"probe"`
	Metric       string   `json:"metric"`
	Unit         string   `json:"unit"`
	Better       string   `json:"better"`
	Base         *Stats   `json:"base"`
	Head         *Stats   `json:"head"`
	Delta        *float64 `json:"delta"`
	DeltaPercent *float64 `json:"delta_percent"`
	WithinNoise  bool     `json:"within_noise"`
}

type Stats struct {
	Samples int     `json:"samples"`
	Median  float64 `json:"median"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

type metricKey struct{ probe, metric string }

func buildReport(opts Options, top, recipe, results, baseCommit, headCommit string, runs []runResult) Report {
	report := Report{
		Repo:       top,
		Recipe:     recipe,
		Base:       Ref{Ref: opts.Base, Commit: baseCommit},
		Head:       Ref{Ref: opts.Head, Commit: headCommit},
		Runs:       opts.Runs,
		Results:    results,
		FailedRuns: []FailedRun{},
		Metrics:    []Metric{},
	}
	values := map[metricKey]map[string][]float64{}
	first := map[metricKey]sample{}
	for _, run := range runs {
		counts := func(c *SideCounts) *int {
			if run.side == SideBase {
				return &c.Base
			}
			return &c.Head
		}
		*counts(&report.Ignored) += run.ignored
		if run.failed {
			*counts(&report.Failed)++
			stderr := filepath.Join(run.dir, "stderr.log")
			report.FailedRuns = append(report.FailedRuns, FailedRun{
				Side:       run.side,
				Run:        run.index,
				ExitCode:   run.exitCode,
				Stderr:     stderr,
				StderrTail: tail(stderr, tailLines),
			})
			continue
		}
		for _, s := range run.samples {
			key := metricKey{s.probe, s.metric}
			if _, ok := first[key]; !ok {
				first[key] = s
				values[key] = map[string][]float64{}
			}
			values[key][run.side] = append(values[key][run.side], s.value)
		}
	}
	keys := make([]metricKey, 0, len(first))
	for key := range first {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b metricKey) int {
		if c := strings.Compare(a.probe, b.probe); c != 0 {
			return c
		}
		return strings.Compare(a.metric, b.metric)
	})
	for _, key := range keys {
		s := first[key]
		m := Metric{Probe: key.probe, Metric: key.metric, Unit: s.unit, Better: s.better}
		m.Base = stats(values[key][SideBase])
		m.Head = stats(values[key][SideHead])
		if m.Base != nil && m.Head != nil {
			delta := m.Head.Median - m.Base.Median
			m.Delta = &delta
			if m.Base.Median != 0 {
				percent := delta / math.Abs(m.Base.Median) * 100
				m.DeltaPercent = &percent
			}
			m.WithinNoise = m.Base.Min <= m.Head.Max && m.Head.Min <= m.Base.Max
		}
		report.Metrics = append(report.Metrics, m)
	}
	return report
}

func stats(values []float64) *Stats {
	if len(values) == 0 {
		return nil
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := len(sorted)
	median := sorted[n/2]
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return &Stats{Samples: n, Median: median, Min: sorted[0], Max: sorted[n-1]}
}

func tail(path string, lines int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) == 1 && all[0] == "" {
		return []string{}
	}
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return all
}

func EncodeJSON(w io.Writer, report Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func EncodeText(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "repo     %s\n", r.Repo)
	fmt.Fprintf(&b, "base     %s  (%s)\n", r.Base.Commit, r.Base.Ref)
	fmt.Fprintf(&b, "head     %s  (%s)\n", r.Head.Commit, r.Head.Ref)
	fmt.Fprintf(&b, "recipe   %s, taken from head\n", r.Recipe)
	fmt.Fprintf(&b, "runs     %d each, interleaved base then head\n", r.Runs)
	fmt.Fprintf(&b, "failed   base %d/%d, head %d/%d\n", r.Failed.Base, r.Runs, r.Failed.Head, r.Runs)
	fmt.Fprintf(&b, "ignored  stdout lines that are not metrics: base %d, head %d\n", r.Ignored.Base, r.Ignored.Head)
	fmt.Fprintf(&b, "results  %s\n", r.Results)
	for _, f := range r.FailedRuns {
		fmt.Fprintf(&b, "\n%s run %d exited %d; stderr in %s\n", f.Side, f.Run, f.ExitCode, f.Stderr)
		for _, line := range f.StderrTail {
			fmt.Fprintf(&b, "  | %s\n", line)
		}
	}
	b.WriteString("\n")
	if len(r.Metrics) == 0 {
		b.WriteString("no metrics from any completed run\n")
		_, err := io.WriteString(w, b.String())
		return err
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "PROBE\tMETRIC\tUNIT\tBETTER\tBASE MEDIAN\tBASE MIN..MAX\tHEAD MEDIAN\tHEAD MIN..MAX\tDELTA\tDELTA %\tNOISE")
	for _, m := range r.Metrics {
		noise := ""
		if m.WithinNoise {
			noise = "within noise"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.Probe, m.Metric, m.Unit, m.Better,
			median(m.Base), span(m.Base), median(m.Head), span(m.Head),
			signed(m.Delta), percent(m.DeltaPercent), noise)
	}
	return table.Flush()
}

func median(s *Stats) string {
	if s == nil {
		return "-"
	}
	return number(s.Median)
}

func span(s *Stats) string {
	if s == nil {
		return "-"
	}
	return fmt.Sprintf("%s..%s (n=%d)", number(s.Min), number(s.Max), s.Samples)
}

func signed(v *float64) string {
	if v == nil {
		return "-"
	}
	if *v > 0 {
		return "+" + number(*v)
	}
	return number(*v)
}

func percent(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%+.1f%%", *v)
}

func number(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 6, 64), 64)
	if err != nil {
		rounded = v
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
