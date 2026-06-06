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
		if res.Issue == nil {
			if r.verbose {
				fmt.Printf("[ OK ] %s (%s)\n", res.CiteName, res.DOI)
			}
			continue
		}
		sev := res.Issue.Kind.severity()
		if res.DOI != "" {
			fmt.Printf("[%-4s] %s (%s): %s: %s\n",
				sev, res.CiteName, res.DOI, res.Issue.Kind, res.Issue.Message)
		} else {
			fmt.Printf("[%-4s] %s: %s\n", sev, res.CiteName, res.Issue.Kind)
		}
	}
	r.printSummary()
}

func (r *Reporter) printSummary() {
	total := len(r.results)
	var warns, errs int
	for _, res := range r.results {
		if res.Issue == nil {
			continue
		}
		if res.Issue.Kind.severity() == "WARN" {
			warns++
		} else {
			errs++
		}
	}
	ok := total - warns - errs
	fmt.Printf("\nChecked %d entries: %d OK, %d warning(s), %d error(s)\n",
		total, ok, warns, errs)
}
