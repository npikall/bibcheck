package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nickng/bibtex"
)

type IssueKind int

const (
	IssueNoDOI IssueKind = iota
	IssueInvalidDOI
	IssueDOINotFound
	IssueNetworkError
)

var IssueName = map[IssueKind]string{
	IssueNoDOI:        "no DOI field",
	IssueInvalidDOI:   "invalid DOI",
	IssueDOINotFound:  "DOI not found",
	IssueNetworkError: "network error",
}

func (k IssueKind) String() string {
	msg, found := IssueName[k]
	if !found {
		return "unknown issue"
	}
	return msg
}

func (k IssueKind) severity() string {
	if k == IssueNoDOI {
		return "WARN"
	}
	return "ERRO"
}

type Issue struct {
	Kind    IssueKind
	Message string
}

type httpResult struct {
	statusCode int
	err        error
}

type EntryResult struct {
	CiteName string
	DOI      string
	Issue    *Issue
}

type job struct {
	citeName string
	doi      string
}

func normalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	return doi
}

func worker(config *Config, jobCh <-chan job, resCh chan<- EntryResult) {
	for j := range jobCh {
		resCh <- checkDOIWithRetry(config, j)
	}
}

func checkDOIWithRetry(config *Config, j job) EntryResult {
	backoff := 100 * time.Millisecond
	for {
		res := checkDOI(config, j.doi)
		if res.err != nil {
			return EntryResult{
				CiteName: j.citeName,
				DOI:      j.doi,
				Issue:    &Issue{Kind: IssueNetworkError, Message: res.err.Error()},
			}
		}
		switch res.statusCode {
		case http.StatusOK:
			return EntryResult{CiteName: j.citeName, DOI: j.doi}
		case http.StatusTooManyRequests:
			time.Sleep(backoff)
			backoff *= 2
			continue
		case http.StatusNotFound:
			return EntryResult{
				CiteName: j.citeName,
				DOI:      j.doi,
				Issue:    &Issue{Kind: IssueDOINotFound, Message: "HTTP 404"},
			}
		default:
			return EntryResult{
				CiteName: j.citeName,
				DOI:      j.doi,
				Issue:    &Issue{Kind: IssueDOINotFound, Message: fmt.Sprintf("HTTP %d", res.statusCode)},
			}
		}
	}
}

func checkDOI(c *Config, doi string) httpResult {
	req, err := http.NewRequest(http.MethodHead, "https://api.crossref.org/works/"+doi, nil)
	if err != nil {
		return httpResult{err: fmt.Errorf("build request: %w", err)}
	}
	if c.email != "" {
		req.Header.Set("User-Agent", fmt.Sprintf("bibcheck/1.0 (mailto:%s)", c.email))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return httpResult{err: fmt.Errorf("http: %w", err)}
	}
	defer resp.Body.Close()
	return httpResult{statusCode: resp.StatusCode}
}

func processBibEntries(config *Config, entries []*bibtex.BibEntry) chan EntryResult {
	numWorkers := min(config.nWorker, runtime.NumCPU())
	numJobs := len(entries)

	jobCh := make(chan job, numJobs)
	resCh := make(chan EntryResult, numJobs)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			worker(config, jobCh, resCh)
		})
	}
	go func() {
		wg.Wait()
		close(resCh)
	}()

	for _, entry := range entries {
		processEntry(jobCh, resCh, entry)
	}
	close(jobCh)
	return resCh
}

func processEntry(jobCh chan<- job, resCh chan<- EntryResult, entry *bibtex.BibEntry) {
	doiField, found := entry.Fields["doi"]
	if !found {
		resCh <- EntryResult{
			CiteName: entry.CiteName,
			Issue:    &Issue{Kind: IssueNoDOI},
		}
		return
	}
	doi := normalizeDOI(doiField.String())
	if doi == "" {
		resCh <- EntryResult{
			CiteName: entry.CiteName,
			Issue:    &Issue{Kind: IssueInvalidDOI, Message: "empty after normalization"},
		}
		return
	}
	jobCh <- job{citeName: entry.CiteName, doi: doi}
}
