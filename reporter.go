package main

import "fmt"

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
				fmt.Printf("[ OK ] %s (%s)\n", res.CiteName, res.DOI)
			}
			continue
		}
		for _, issue := range res.Issues {
			sev := issue.Kind.severity()
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
				fmt.Printf("[%-4s] %s (%s): %s: %s\n", sev, res.CiteName, ref, issue.Kind, issue.Message)
			} else if ref != "" {
				fmt.Printf("[%-4s] %s (%s): %s\n", sev, res.CiteName, ref, issue.Kind)
			} else if issue.Message != "" {
				fmt.Printf("[%-4s] %s: %s: %s\n", sev, res.CiteName, issue.Kind, issue.Message)
			} else {
				fmt.Printf("[%-4s] %s: %s\n", sev, res.CiteName, issue.Kind)
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
	fmt.Printf("\nChecked %d entries: %d OK, %d warning(s), %d error(s)\n",
		total, ok, warns, errs)
}
