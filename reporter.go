package main

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

var (
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("192")).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
	citeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
)

func severityStyle(sev string) string {
	switch sev {
	case "WARN":
		return warnStyle.Render(fmt.Sprintf("%-4s", sev))
	default:
		return errorStyle.Render(fmt.Sprintf("%-4s", sev))
	}
}

type Reporter struct {
	results []EntryResult
	verbose bool
}

func (r *Reporter) Collect(res EntryResult) {
	r.results = append(r.results, res)
}

func (r *Reporter) Print() {
	for _, res := range r.results {
		if len(res.Issues) == 0 {
			if r.verbose {
				fmt.Printf("[ OK ] %s (%s)\n", citeStyle.Render(res.CiteName), res.DOI)
			}
			continue
		}
		cite := citeStyle.Render(res.CiteName)
		for _, issue := range res.Issues {
			sev := severityStyle(issue.Kind.severity())
			var ref string
			switch issue.Kind {
			case IssueURLDead, IssueURLError:
				ref = res.URL
			case IssueNoDOI, IssueInvalidDOI:
				ref = ""
			default:
				ref = res.DOI
			}
			if ref != "" && issue.Message != "" {
				fmt.Printf("[%s] %s (%s): %s: %s\n", sev, cite, ref, issue.Kind, issue.Message)
			} else if ref != "" {
				fmt.Printf("[%s] %s (%s): %s\n", sev, cite, ref, issue.Kind)
			} else if issue.Message != "" {
				fmt.Printf("[%s] %s: %s: %s\n", sev, cite, issue.Kind, issue.Message)
			} else {
				fmt.Printf("[%s] %s: %s\n", sev, cite, issue.Kind)
			}
		}
	}
	r.printSummary()
}

func (r *Reporter) printSummary() {
	total := len(r.results)
	var warns, errs, withIssues int
	for _, res := range r.results {
		if len(res.Issues) > 0 {
			withIssues++
		}
		for _, issue := range res.Issues {
			if issue.Kind.severity() == "WARN" {
				warns++
			} else {
				errs++
			}
		}
	}
	ok := total - withIssues
	fmt.Printf("\nChecked %d entries: %d OK, %s, %s\n",
		total, ok,
		warnStyle.Render(fmt.Sprintf("%d warning(s)", warns)),
		errorStyle.Render(fmt.Sprintf("%d error(s)", errs)),
	)
}
