package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nickng/bibtex"
)

type Config struct {
	client  *http.Client
	email   string
	verbose bool
	nWorker int
}

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

func main() {
	email := flag.String("email", "", "An email to get better rate limits from Crossref API")
	nWorker := flag.Int("n", 1, "Number of workers for concurrent processing")
	verbose := flag.Bool("v", false, "Produce verbose output")
	flag.Parse()
	config := NewConfig(*email, *verbose, *nWorker)

	file := resolveArgs()
	ext := filepath.Ext(file)
	switch ext {
	case ".yml", ".yaml":
		fmt.Fprintln(os.Stderr, "yaml not yet supported")
		os.Exit(1)
	case ".bib":
		if err := processBibFile(file, config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown file extension:", ext)
		os.Exit(1)
	}
}

func processBibFile(file string, config *Config) error {
	bib, err := parseBibFile(file)
	if err != nil {
		return err
	}

	spinner := &Spinner{}
	spinner.Start(len(bib.Entries))

	reporter := &Reporter{verbose: config.verbose}
	resCh := processBibEntries(config, bib.Entries)
	for res := range resCh {
		spinner.Increment()
		reporter.Collect(res)
	}
	spinner.Stop()
	reporter.Print()
	return nil
}

func resolveArgs() string {
	if flag.Arg(0) == "help" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "bibcheck [-email string][-n int] <file>")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}
	return flag.Arg(0)
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

func NewConfig(email string, verbose bool, n int) *Config {
	return &Config{
		client:  &http.Client{Timeout: 5 * time.Second},
		email:   email,
		verbose: verbose,
		nWorker: n,
	}
}

func parseBibFile(file string) (*bibtex.BibTex, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", file, err)
	}
	defer f.Close()
	bib, err := bibtex.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	return bib, nil
}

func normalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	return doi
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
