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

var logger = log.Default()

type customClient struct {
	client *http.Client
	email  string
}

func main() {
	email := flag.String("email", "", "An email to get better rate limits from Crossref API")
	flag.Parse()

	file := "refs.bib"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}
	bib := parseBibFile(file)

	client := &customClient{
		client: &http.Client{Timeout: 5 * time.Second},
		email:  *email,
	}

	numWorkers := min(runtime.NumCPU(), 8)
	numJobs := len(bib.Entries)
	doiCh := make(chan string, numJobs)
	resCh := make(chan string, numJobs)
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			worker(client, doiCh, resCh)
		})
	}
	go func() {
		wg.Wait()
		close(resCh)
	}()

	for _, entry := range bib.Entries {
		doiField, found := entry.Fields["doi"]
		if !found {
			logger.Println("[WARN] no doi found:", entry.CiteName)
			continue
		}

		doi := normalizeDOI(doiField.String())
		doiCh <- doi
	}
	close(doiCh)

	for result := range resCh {
		logger.Println(result)
	}
}

func worker(client *customClient, doiCh <-chan string, resCh chan<- string) {
	for doi := range doiCh {
		resCh <- checkDOIWithRetry(client, doi)
	}
}

func checkDOIWithRetry(client *customClient, doi string) string {
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

func checkDOI(c *customClient, doi string) *result {
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

	return &result{status: resp.Status, statusCode: resp.StatusCode}
}

type result struct {
	status     string
	statusCode int
}

func parseBibFile(file string) *bibtex.BibTex {
	f, err := os.Open(file)
	if err != nil {
		logger.Fatal(err)
	}

	bib, err := bibtex.Parse(f)
	if err != nil {
		logger.Fatal(err)
	}
	return bib
}

func normalizeDOI(doi string) string {
	doi = strings.TrimSpace(doi)
	doi = strings.TrimPrefix(doi, "http://doi.org/")
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	return doi
}
