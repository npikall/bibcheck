package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strconv"
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
	IssueURLForbidden
	IssueDiff
)

var IssueName = map[IssueKind]string{
	IssueNoDOI:        "no DOI field",
	IssueInvalidDOI:   "invalid DOI",
	IssueDOINotFound:  "DOI not found",
	IssueNetworkError: "network error",
	IssueURLDead:      "URL dead",
	IssueURLError:     "URL server error",
	IssueURLForbidden: "URL unverifiable (bot-protected)",
	IssueDiff:         "field mismatch",
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
	LevelDiff  = "DIFF"
)

func (k IssueKind) severity() string {
	switch k { //nolint:exhaustive
	case IssueNoDOI, IssueURLError, IssueURLForbidden:
		return LevelWarn
	case IssueDiff:
		return LevelDiff
	default:
		return LevelError
	}
}

type Issue struct {
	Kind    IssueKind
	Message string
	Detail  string
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
	author      string
	title       string
	year        string
	entryType   string
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
		doiIssues := checkDOIWithRetry(config, j.doi)
		issues = append(issues, doiIssues...)
		if len(doiIssues) == 0 && config.verify {
			issues = append(issues, checkMetadata(config, j)...)
		}
	}

	if j.url != "" && config.checkURLs {
		issues = append(issues, checkURLWithRetry(config, j.url)...)
	}

	return EntryResult{CiteName: j.citeName, DOI: j.doi, URL: j.url, Issues: issues}
}

func checkDOIWithRetry(config *Config, doi string) []Issue {
	backoff := 100 * time.Millisecond //nolint:mnd
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
	url := c.crossrefBaseURL + doi
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, url, nil)
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
	statusCode := resp.StatusCode
	if err := resp.Body.Close(); err != nil {
		return httpResult{err: fmt.Errorf("close response body: %w", err)}
	}
	return httpResult{statusCode: statusCode}
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
	case res.statusCode == http.StatusForbidden:
		return []Issue{{Kind: IssueURLForbidden, Message: "HTTP 403"}}
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
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		return httpResult{err: fmt.Errorf("build request: %w", err)}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return httpResult{err: fmt.Errorf("http: %w", err)}
	}
	statusCode := resp.StatusCode
	if err := resp.Body.Close(); err != nil {
		return httpResult{err: fmt.Errorf("close response body: %w", err)}
	}
	return httpResult{statusCode: statusCode}
}

// crossref API response types for metadata verification.
type crossrefResponse struct {
	Message crossrefMessage `json:"message"`
}

type crossrefMessage struct {
	Title     []string         `json:"title"`
	Author    []crossrefAuthor `json:"author"`
	Published crossrefDate     `json:"published"`
}

type crossrefAuthor struct {
	Family string `json:"family"`
	Given  string `json:"given"`
}

type crossrefDate struct {
	DateParts [][]int `json:"date-parts"`
}

func (d crossrefDate) year() string {
	if len(d.DateParts) > 0 && len(d.DateParts[0]) > 0 {
		return strconv.Itoa(d.DateParts[0][0])
	}
	return ""
}

const (
	entryArticle       = "article"
	entryBook          = "book"
	entryInproceedings = "inproceedings"
	entryMisc          = "misc"

	fieldAuthor = "author"
	fieldTitle  = "title"
	fieldYear   = "year"
)

var (
	allFields   = []string{fieldAuthor, fieldTitle, fieldYear}
	withoutYear = []string{fieldAuthor, fieldTitle}
	titleOnly   = []string{fieldTitle}
)

// bibRequiredFields maps BibTeX entry types to the subset of {author, title, year} that are required.
var bibRequiredFields = map[string][]string{
	entryArticle:       allFields,
	entryBook:          allFields,
	"booklet":          titleOnly,
	"conference":       allFields,
	"inbook":           allFields,
	"incollection":     allFields,
	entryInproceedings: allFields,
	"manual":           titleOnly,
	"mastersthesis":    allFields,
	entryMisc:          {},
	"online":           allFields,
	"phdthesis":        allFields,
	"proceedings":      {fieldTitle, fieldYear},
	"techreport":       allFields,
	"unpublished":      withoutYear,
}

// hayagrivaTypeToBib maps Hayagriva entry types to BibTeX equivalents.
var hayagrivaTypeToBib = map[string]string{
	entryArticle:  entryArticle,
	entryBook:     entryBook,
	"chapter":     "inbook",
	"proceedings": entryInproceedings,
	"thesis":      "phdthesis",
	"web":         entryMisc,
	"newspaper":   entryArticle,
	"magazine":    entryArticle,
	"report":      "techreport",
	"blog":        entryMisc,
	"video":       entryMisc,
	"audio":       entryMisc,
	"patent":      entryMisc,
	"conference":  entryInproceedings,
	"anthology":   "incollection",
}

func resolveEntryType(t string) string {
	t = strings.ToLower(t)
	if mapped, ok := hayagrivaTypeToBib[t]; ok {
		return mapped
	}
	return t
}

func isRequired(entryType, field string) bool {
	fields, ok := bibRequiredFields[strings.ToLower(entryType)]
	if !ok {
		return false
	}
	return slices.Contains(fields, field)
}

func normalizeTitle(s string) string {
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func extractBibFamilyNames(author string) []string {
	parts := strings.Split(author, " and ")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var family string
		if before, _, found := strings.Cut(part, ","); found {
			family = strings.TrimSpace(before)
		} else {
			words := strings.Fields(part)
			if len(words) > 0 {
				family = words[len(words)-1]
			}
		}
		if family != "" {
			names = append(names, strings.ToLower(family))
		}
	}
	return names
}

func extractCrossRefFamilyNames(authors []crossrefAuthor) []string {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		if a.Family != "" {
			names = append(names, strings.ToLower(a.Family))
		}
	}
	return names
}

func familyNameSetsEqual(local, remote []string) bool {
	if len(local) != len(remote) {
		return false
	}
	set := make(map[string]struct{}, len(local))
	for _, v := range local {
		set[v] = struct{}{}
	}
	for _, v := range remote {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}

func formatCrossRefAuthors(authors []crossrefAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		if a.Given != "" {
			parts = append(parts, a.Family+", "+a.Given)
		} else {
			parts = append(parts, a.Family)
		}
	}
	return strings.Join(parts, " and ")
}

func fetchCrossRefMessage(config *Config, doi string) (crossrefMessage, bool) {
	url := config.crossrefBaseURL + doi
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return crossrefMessage{}, false
	}
	if config.email != "" {
		req.Header.Set("User-Agent", fmt.Sprintf("bibcheck/1.0 (mailto:%s)", config.email))
	}
	resp, err := config.client.Do(req)
	if err != nil {
		return crossrefMessage{}, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return crossrefMessage{}, false
	}
	var cr crossrefResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return crossrefMessage{}, false
	}
	return cr.Message, true
}

func (j job) diffTitle(entryType string, msg crossrefMessage) *Issue {
	if !isRequired(entryType, fieldTitle) {
		return nil
	}
	if j.title == "" || len(msg.Title) == 0 {
		return nil
	}
	if normalizeTitle(j.title) == normalizeTitle(msg.Title[0]) {
		return nil
	}
	return &Issue{
		Kind:    IssueDiff,
		Message: "title mismatch",
		Detail:  fmt.Sprintf("         local:    %q\n         crossref: %q", j.title, msg.Title[0]),
	}
}

func (j job) diffAuthor(entryType string, msg crossrefMessage) *Issue {
	if !isRequired(entryType, fieldAuthor) {
		return nil
	}
	if j.author == "" || len(msg.Author) == 0 {
		return nil
	}
	if familyNameSetsEqual(extractBibFamilyNames(j.author), extractCrossRefFamilyNames(msg.Author)) {
		return nil
	}
	return &Issue{
		Kind:    IssueDiff,
		Message: "author mismatch",
		Detail:  fmt.Sprintf("         local:    %q\n         crossref: %q", j.author, formatCrossRefAuthors(msg.Author)),
	}
}

func (j job) diffYear(entryType string, msg crossrefMessage) *Issue {
	crYear := msg.Published.year()
	if !isRequired(entryType, fieldYear) || j.year == "" || crYear == "" || j.year == crYear {
		return nil
	}
	return &Issue{
		Kind:    IssueDiff,
		Message: "year mismatch",
		Detail:  fmt.Sprintf("         local:    %q\n         crossref: %q", j.year, crYear),
	}
}

func checkMetadata(config *Config, j job) []Issue {
	msg, ok := fetchCrossRefMessage(config, j.doi)
	if !ok {
		return nil
	}
	entryType := resolveEntryType(j.entryType)
	var issues []Issue
	for _, issue := range []*Issue{
		j.diffTitle(entryType, msg),
		j.diffAuthor(entryType, msg),
		j.diffYear(entryType, msg),
	} {
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues
}

// Parser converts a bibliography source into jobs for the processing pipeline.
type Parser interface {
	Parse(r io.Reader) ([]job, error)
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
