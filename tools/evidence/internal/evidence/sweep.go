package evidence

import (
	"cmp"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"
)

const (
	redactedValue = "REDACTED"

	sweepBudget = 30 * time.Second

	reasonNoSweep  = "gitleaks is not installed and there is no sweep without it, so nothing was scanned and nothing from the reviewed code is printed"
	reasonWithheld = "the report could not be swept for secret values, so what this check found is withheld"
	reasonUnplaced = "gitleaks named a value that could not be placed in the report"
	reasonSwept    = "every string went through gitleaks and what it recognised is replaced; this covers what gitleaks recognises, and a value it does not recognise stays as it was"
	reasonUnread   = reasonSwept + "; the added lines could not be read, so a value known only from the keyword beside it there is not replaced"
)

func unswept(reason string) Report {
	return Report{
		Secrets:      notRun(reason),
		Dependencies: notRun(reasonNoSweep),
		Workflows:    notRun(reasonNoSweep),
		Sweep:        Sweep{Status: StatusNotRun, Reason: reasonNoSweep},
	}
}

func (s *session) sweep(ctx context.Context, report Report) Report {
	path, err := lookup(gitleaksName)
	if err != nil {
		return withhold(report, reasonNoSweep)
	}
	ctx, cancel := context.WithTimeout(ctx, sweepBudget)
	defer cancel()

	var texts []string
	eachString(&report, func(text *string) {
		texts = append(texts, *text)
	})
	leaks, err := s.readLines(ctx, path, recognisableLines(texts...), 0, false)
	if err != nil {
		return withhold(report, err.Error())
	}
	if err := redactValues(&report, leaks, s.values); err != nil {
		return withhold(report, err.Error())
	}
	report.Sweep = Sweep{Status: StatusRan, Reason: reasonSwept}
	if report.Secrets.Status != StatusRan {
		report.Sweep.Reason = reasonUnread
	}
	return report
}

func redactValues(report *Report, leaks []gitleaksLeak, known []string) error {
	values := slices.Clone(known)
	for _, leak := range leaks {
		if leak.Secret == "" {
			return errors.New(reasonUnplaced)
		}
		values = append(values, leak.Secret)
	}
	slices.SortFunc(values, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b))
	})
	values = slices.Compact(values)

	placed := map[string]bool{}
	eachString(report, func(text *string) {
		original := *text
		for _, value := range values {
			if strings.Contains(original, value) {
				placed[value] = true
				*text = strings.ReplaceAll(*text, value, redactedValue)
			}
		}
	})
	for _, leak := range leaks {
		if !placed[leak.Secret] {
			return errors.New(reasonUnplaced)
		}
	}
	return nil
}
func withhold(report Report, reason string) Report {
	return Report{
		Secrets:      withheld(report.Secrets),
		Dependencies: withheld(report.Dependencies),
		Workflows:    withheld(report.Workflows),
		Sweep:        Sweep{Status: StatusNotRun, Reason: reason},
	}
}

func withheld(check Check) Check {
	kept := Check{Status: check.Status, Reason: check.Reason, Scanners: []Scanner{}, New: []Finding{}}
	if check.Status == StatusRan || check.Status == StatusPartial {
		kept.Status = StatusNotRun
		kept.Reason = reasonWithheld
	}
	return kept
}

func eachString(report *Report, visit func(*string)) {
	walkStrings(reflect.ValueOf(report).Elem(), visit)
}

func walkStrings(value reflect.Value, visit func(*string)) {
	switch value.Kind() {
	case reflect.String:
		visit(value.Addr().Interface().(*string))
	case reflect.Struct:
		for index := range value.NumField() {
			walkStrings(value.Field(index), visit)
		}
	case reflect.Slice:
		for index := range value.Len() {
			walkStrings(value.Index(index), visit)
		}
	}
}
