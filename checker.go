package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

type IssueKind int

const (
	IssueNoDOI IssueKind = iota
	IssueInvalidDOI
	IssueDOINotFound
	IssueNetworkError
	IssueURLDead
	IssueURLError
)

var IssueName = map[IssueKind]string{
	IssueNoDOI:        "no DOI field",
	IssueInvalidDOI:   "invalid DOI",
	IssueDOINotFound:  "DOI not found",
	IssueNetworkError: "network error",
	IssueURLDead:      "URL dead",
	IssueURLError:     "URL server error",
}

func (k IssueKind) String() string {
	msg, found := IssueName[k]
	if !found {
		return "unknown issue"
	}
	return msg
}

const (
	LevelWarn  = "WARN"
	LevelError = "ERRO"
)

func (k IssueKind) severity() string {
	switch k { //nolint:  exhaustive
	case IssueNoDOI, IssueURLError:
		return LevelWarn
	default:
		return LevelError
	}
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
	URL      string
	Issues   []Issue
}

type job struct {
	citeName    string
	doi         string
	url         string
	localIssues []Issue
}

func normalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	return doi
}

func worker(config *Config, jobCh <-chan job, resCh chan<- EntryResult) {
	for j := range jobCh {
		resCh <- processJob(config, j)
	}
}

func processJob(config *Config, j job) EntryResult {
	issues := append([]Issue{}, j.localIssues...)

	if j.doi != "" {
		issues = append(issues, checkDOIWithRetry(config, j.doi)...)
	}

	if j.url != "" {
		issues = append(issues, checkURLWithRetry(config, j.url)...)
	}

	return EntryResult{CiteName: j.citeName, DOI: j.doi, URL: j.url, Issues: issues}
}

func checkDOIWithRetry(config *Config, doi string) []Issue {
	backoff := 100 * time.Millisecond //nolint: mnd
	for attempt := range config.maxRetries {
		res := checkDOI(config, doi)
		if res.err != nil {
			return []Issue{{Kind: IssueNetworkError, Message: res.err.Error()}}
		}
		switch res.statusCode {
		case http.StatusOK:
			return nil
		case http.StatusTooManyRequests:
			if attempt == config.maxRetries {
				return []Issue{{Kind: IssueNetworkError, Message: fmt.Sprintf("rate limited after %d retries", config.maxRetries)}}
			}
			time.Sleep(backoff)
			backoff *= 2
		case http.StatusNotFound:
			return []Issue{{Kind: IssueDOINotFound, Message: "HTTP 404"}}
		default:
			return []Issue{{Kind: IssueDOINotFound, Message: fmt.Sprintf("HTTP %d", res.statusCode)}}
		}
	}
	return []Issue{{Kind: IssueNetworkError, Message: fmt.Sprintf("rate limited after %d retries", config.maxRetries)}}
}

func checkDOI(c *Config, doi string) httpResult {
	url := "https://api.crossref.org/works/" + doi
	req, err := http.NewRequest(http.MethodHead, url, nil)
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

func checkURLWithRetry(c *Config, rawURL string) []Issue {
	res := checkURL(c, rawURL)
	if res.err != nil {
		res = checkURL(c, rawURL)
		if res.err != nil {
			return []Issue{{Kind: IssueNetworkError, Message: res.err.Error()}}
		}
	}
	switch {
	case res.statusCode >= http.StatusOK && res.statusCode < http.StatusMultipleChoices:
		return nil
	case res.statusCode >= http.StatusBadRequest && res.statusCode < http.StatusInternalServerError:
		return []Issue{{Kind: IssueURLDead, Message: fmt.Sprintf("HTTP %d", res.statusCode)}}
	case res.statusCode >= http.StatusInternalServerError:
		return []Issue{{Kind: IssueURLError, Message: fmt.Sprintf("HTTP %d", res.statusCode)}}
	default:
		return []Issue{{Kind: IssueURLDead, Message: fmt.Sprintf("HTTP %d", res.statusCode)}}
	}
}

func checkURL(c *Config, rawURL string) httpResult {
	res := makeRequest(c, http.MethodHead, rawURL)
	if res.statusCode == http.StatusMethodNotAllowed || res.statusCode == http.StatusForbidden {
		return makeRequest(c, http.MethodGet, rawURL)
	}
	return res
}

func makeRequest(c *Config, method string, url string) httpResult {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return httpResult{err: fmt.Errorf("build request: %w", err)}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return httpResult{err: fmt.Errorf("http: %w", err)}
	}
	defer resp.Body.Close()

	return httpResult{statusCode: resp.StatusCode}
}

// Parser converts a bibliography file into jobs for the processing pipeline.
type Parser interface {
	Parse(file string) ([]job, error)
}

func processJobs(config *Config, jobs []job) chan EntryResult {
	numWorkers := min(config.nWorker, runtime.NumCPU())

	jobCh := make(chan job, len(jobs))
	resCh := make(chan EntryResult, len(jobs))

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

	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	return resCh
}
