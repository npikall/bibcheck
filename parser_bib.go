package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/nickng/bibtex"
)

type BibParser struct{}

func (p *BibParser) Parse(file string) (_ []job, err error) {
	f, err := os.Open(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	bib, err := bibtex.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	jobs := make([]job, 0, len(bib.Entries))
	for _, entry := range bib.Entries {
		j := job{citeName: entry.CiteName}

		if doiField, found := entry.Fields["doi"]; found {
			doi := normalizeDOI(doiField.String())
			if doi == "" {
				j.localIssues = append(j.localIssues, Issue{Kind: IssueInvalidDOI, Message: "empty after normalization"})
			} else {
				j.doi = doi
			}
		} else {
			j.localIssues = append(j.localIssues, Issue{Kind: IssueNoDOI})
		}

		if urlField, found := entry.Fields["url"]; found {
			j.url = strings.TrimSpace(urlField.String())
		}

		jobs = append(jobs, j)
	}
	return jobs, nil
}
