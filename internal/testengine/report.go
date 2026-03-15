package testengine

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ANSI color codes consistent with internal/output/human.go.
const (
	resetCode = "\033[0m"
	redCode   = "\033[31m"
	greenCode = "\033[32m"
)

// Summary computes totals across all files in the report.
func Summary(report *TestReport) (total, passed, failed, errors int) {
	for _, f := range report.Files {
		passed += f.Passed
		failed += f.Failed
		errors += f.Errors
	}
	total = passed + failed + errors
	return
}

// RenderHuman writes a human-readable test report to w.
func RenderHuman(w io.Writer, report *TestReport, useColor bool) {
	fmt.Fprintln(w, "=== cactl test ===")
	fmt.Fprintln(w)

	for _, f := range report.Files {
		fmt.Fprintf(w, "  %s\n", f.File)
		for _, s := range f.Scenarios {
			if s.Error != "" {
				renderStatus(w, useColor, false, s.ScenarioName)
				fmt.Fprintf(w, "          error: %s\n", s.Error)
				continue
			}
			if s.Passed {
				matchInfo := fmt.Sprintf("%d policies matched, result: %s",
					len(s.Got.MatchingPolicies), s.Got.Result.String())
				renderStatus(w, useColor, true, fmt.Sprintf("%s (%s)", s.ScenarioName, matchInfo))
			} else {
				renderStatus(w, useColor, false, s.ScenarioName)
				fmt.Fprintf(w, "          expected: %s\n", s.Expected.Result)
				gotStr := s.Got.Result.String()
				if len(s.Got.GrantControls) > 0 {
					gotStr += " [" + strings.Join(s.Got.GrantControls, ", ") + "]"
				}
				fmt.Fprintf(w, "          got:      %s\n", gotStr)
				if len(s.MatchingPolicies) > 0 {
					fmt.Fprintf(w, "          matching policies: %s\n", strings.Join(s.MatchingPolicies, ", "))
				}
			}
		}
		fmt.Fprintln(w)
	}

	total, passed, failed, errors := Summary(report)
	_ = total
	fmt.Fprintf(w, "  Results: %d passed, %d failed, %d errors\n", passed, failed, errors)
}

// renderStatus writes a PASS or FAIL line with optional color.
func renderStatus(w io.Writer, useColor, passed bool, msg string) {
	if passed {
		if useColor {
			fmt.Fprintf(w, "    %sPASS%s  %s\n", greenCode, resetCode, msg)
		} else {
			fmt.Fprintf(w, "    PASS  %s\n", msg)
		}
	} else {
		if useColor {
			fmt.Fprintf(w, "    %sFAIL%s  %s\n", redCode, resetCode, msg)
		} else {
			fmt.Fprintf(w, "    FAIL  %s\n", msg)
		}
	}
}

// jsonReport is the JSON output structure.
type jsonReport struct {
	Summary jsonSummary `json:"summary"`
	Files   []jsonFile  `json:"files"`
}

type jsonSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Errors int `json:"errors"`
}

type jsonFile struct {
	File      string         `json:"file"`
	Scenarios []jsonScenario `json:"scenarios"`
}

type jsonScenario struct {
	Name             string   `json:"name"`
	Passed           bool     `json:"passed"`
	ExpectedResult   string   `json:"expectedResult"`
	GotResult        string   `json:"gotResult"`
	GotControls      []string `json:"gotControls,omitempty"`
	MatchingPolicies []string `json:"matchingPolicies,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// RenderJSON writes a machine-readable JSON test report to w.
func RenderJSON(w io.Writer, report *TestReport) error {
	total, passed, failed, errors := Summary(report)

	jr := jsonReport{
		Summary: jsonSummary{
			Total:  total,
			Passed: passed,
			Failed: failed,
			Errors: errors,
		},
	}

	for _, f := range report.Files {
		jf := jsonFile{File: f.File}
		for _, s := range f.Scenarios {
			js := jsonScenario{
				Name:             s.ScenarioName,
				Passed:           s.Passed,
				ExpectedResult:   s.Expected.Result,
				GotResult:        s.Got.Result.String(),
				GotControls:      s.Got.GrantControls,
				MatchingPolicies: s.MatchingPolicies,
				Error:            s.Error,
			}
			jf.Scenarios = append(jf.Scenarios, js)
		}
		jr.Files = append(jr.Files, jf)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}
