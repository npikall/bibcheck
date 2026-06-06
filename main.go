package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nickng/bibtex"
)

type Client struct {
	client *http.Client
	email  string
}

type Result struct {
	status     string
	statusCode int
}

func main() {
	email := flag.String("email", "", "An email to get better rate limits from Crossref API")
	flag.Parse()

	file := getFilePath()
	bib := parseBibFile(file)

	client := NewClient(*email)
	resCh := processData(client, bib.Entries)

	for result := range resCh {
		fmt.Println(result)
	}
}

func getFilePath() string {
	file := "refs.bib"
	if flag.NArg() > 1 {
		file = flag.Arg(0)
	}
	return file
}

func worker(client *Client, doiCh <-chan string, resCh chan<- string) {
	for doi := range doiCh {
		resCh <- checkDOIWithRetry(client, doi)
	}
}

func checkDOIWithRetry(client *Client, doi string) string {
	backoff := 500 * time.Millisecond
	for {
		res := checkDOI(client, doi)
		if res.statusCode == http.StatusTooManyRequests {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if res.statusCode != http.StatusOK {
			return fmt.Sprintf("[BAD] request %s: %s", doi, res.status)
		}
		return fmt.Sprintf("[OK] %s", doi)
	}
}

func checkDOI(c *Client, doi string) *Result {
	req, err := http.NewRequest(http.MethodHead, "https://api.crossref.org/works/doi"+doi, nil)
	if err != nil {
		return nil
	}
	if c.email != "" {
		req.Header.Set("User-Agent", fmt.Sprintf("bibcheck/1.0 (mailto:%s)", c.email))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	return &Result{status: resp.Status, statusCode: resp.StatusCode}
}

func NewClient(email string) *Client {
	return &Client{
		client: &http.Client{Timeout: 5 * time.Second},
		email:  email,
	}
}

func parseBibFile(file string) *bibtex.BibTex {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}

	bib, err := bibtex.Parse(f)
	if err != nil {
		log.Fatal(err)
	}
	return bib
}

func normalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	return doi
}

func processData(client *Client, entries []*bibtex.BibEntry) chan string {
	numWorkers := min(runtime.NumCPU()-1, 8)
	numJobs := len(entries)

	jobCh := make(chan string, numJobs)
	resCh := make(chan string, numJobs)

	// Start up worker goroutines
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			worker(client, jobCh, resCh)
		})
	}
	// defer closing result channel
	go func() {
		wg.Wait()
		close(resCh)
	}()

	// send jobs to the workers
	for _, entry := range entries {
		processEntry(jobCh, entry)
	}
	close(jobCh)
	return resCh
}

func processEntry(jobCh chan<- string, entry *bibtex.BibEntry) {
	doiField, found := entry.Fields["doi"]
	if !found {
		fmt.Println("[WARN] no doi found:", entry.CiteName)
		return
	}
	doi := normalizeDOI(doiField.String())
	jobCh <- doi
}
