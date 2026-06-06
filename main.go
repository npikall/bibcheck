package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	client     *http.Client
	email      string
	verbose    bool
	checkURLs  bool
	nWorker    int
	maxRetries int
}

func NewConfig(email string, verbose bool, checkURLs bool, n int, maxRetries int) *Config {
	return &Config{
		client:     &http.Client{Timeout: 5 * time.Second}, //nolint: mnd
		email:      email,
		verbose:    verbose,
		checkURLs:  checkURLs,
		nWorker:    n,
		maxRetries: maxRetries,
	}
}

func main() {
	log.SetOutput(io.Discard)
	email := flag.String("email", "", "An email to get better rate limits from Crossref API")
	nWorker := flag.Int("n", 1, "Number of workers for concurrent processing")
	verbose := flag.Bool("v", false, "Produce verbose output")
	checkURLs := flag.Bool("urls", true, "Check URLs in bibliography entries")
	maxRetries := flag.Int("retry", 3, "Max retries when fetching DOI data on rate limit (429)") //nolint: mnd
	flag.Parse()
	config := NewConfig(*email, *verbose, *checkURLs, *nWorker, *maxRetries)

	file := resolveArgs()
	ext := filepath.Ext(file)

	var parser Parser
	switch ext {
	case ".yml", ".yaml":
		parser = &YAMLParser{}
	case ".bib":
		parser = &BibParser{}
	default:
		fmt.Fprintln(os.Stderr, "unknown file extension:", ext)
		os.Exit(1)
	}

	if err := processFile(file, parser, config); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func processFile(file string, parser Parser, config *Config) error {
	jobs, err := parser.Parse(file)
	if err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}

	spinner := &Spinner{}
	spinner.Start(len(jobs))

	reporter := &Reporter{verbose: config.verbose}
	resCh := processJobs(config, jobs)
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
