package main

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

var (
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("192")).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
	diffStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	citeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
)

func severityStyle(sev string) string {
	switch sev {
	case LevelWarn:
		return warnStyle.Render(fmt.Sprintf("%-4s", sev))
	case LevelDiff:
		return diffStyle.Render(fmt.Sprintf("%-4s", sev))
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
				fmt.Printf("[%s] %s (%s)\n", okStyle.Render(" OK "), citeStyle.Render(res.CiteName), res.DOI)
			}
			continue
		}
		r.printEntry(res)
	}
	r.printSummary()
}

func (r *Reporter) printEntry(res EntryResult) {
	cite := citeStyle.Render(res.CiteName)
	for _, issue := range res.Issues {
		r.printIssue(cite, res, issue)
	}
}

func (r *Reporter) printIssue(cite string, res EntryResult, issue Issue) {
	sev := severityStyle(issue.Kind.severity())
	ref := issueRef(res, issue)

	if issue.Kind == IssueDiff {
		fmt.Printf("[%s] %s %s (%s):\n%s\n", sev, cite, issue.Message, ref, issue.Detail)
		return
	}

	switch {
	case ref != "" && issue.Message != "":
		fmt.Printf("[%s] %s %s (%s): %s\n", sev, cite, issue.Kind, ref, issue.Message)
	case ref != "":
		fmt.Printf("[%s] %s %s (%s)\n", sev, cite, issue.Kind, ref)
	case issue.Message != "":
		fmt.Printf("[%s] %s %s: %s\n", sev, cite, issue.Kind, issue.Message)
	default:
		fmt.Printf("[%s] %s %s\n", sev, cite, issue.Kind)
	}
}

func issueRef(res EntryResult, issue Issue) string {
	switch issue.Kind { //nolint:exhaustive
	case IssueURLDead, IssueURLError, IssueURLForbidden:
		return res.URL
	case IssueNoDOI, IssueInvalidDOI:
		return ""
	default:
		return res.DOI
	}
}

func (r *Reporter) printSummary() {
	total := len(r.results)
	var warns, errs, diffs, withIssues int
	for _, res := range r.results {
		if len(res.Issues) > 0 {
			withIssues++
		}
		for _, issue := range res.Issues {
			switch issue.Kind.severity() {
			case LevelWarn:
				warns++
			case LevelDiff:
				diffs++
			default:
				errs++
			}
		}
	}
	ok := total - withIssues
	line := fmt.Sprintf("\nChecked %d entries: %d OK, %s, %s",
		total, ok,
		warnStyle.Render(fmt.Sprintf("%d warning(s)", warns)),
		errorStyle.Render(fmt.Sprintf("%d error(s)", errs)),
	)
	if diffs > 0 {
		line += ", " + diffStyle.Render(fmt.Sprintf("%d diff(s)", diffs))
	}
	fmt.Println(line)
}
