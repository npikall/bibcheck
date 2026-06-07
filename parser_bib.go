package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/nickng/bibtex"
)

type BibParser struct{}

func (p *BibParser) Parse(r io.Reader) ([]job, error) {
	bib, err := bibtex.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse bib: %w", err)
	}

	jobs := make([]job, 0, len(bib.Entries))
	for _, entry := range bib.Entries {
		j := job{
			citeName:  entry.CiteName,
			entryType: strings.ToLower(entry.Type),
		}

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
		if titleField, found := entry.Fields["title"]; found {
			j.title = strings.TrimSpace(titleField.String())
		}
		if authorField, found := entry.Fields["author"]; found {
			j.author = strings.TrimSpace(authorField.String())
		}
		if yearField, found := entry.Fields["year"]; found {
			j.year = strings.TrimSpace(yearField.String())
		}

		jobs = append(jobs, j)
	}
	return jobs, nil
}
