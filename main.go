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
	client          *http.Client
	email           string
	verbose         bool
	checkURLs       bool
	verify          bool
	nWorker         int
	maxRetries      int
	crossrefBaseURL string
}

func NewConfig(email string, verbose bool, checkURLs bool, verify bool, n int, maxRetries int) *Config {
	return &Config{
		client:          &http.Client{Timeout: 5 * time.Second}, //nolint: mnd
		email:           email,
		verbose:         verbose,
		checkURLs:       checkURLs,
		verify:          verify,
		nWorker:         n,
		maxRetries:      maxRetries,
		crossrefBaseURL: "https://api.crossref.org/works/",
	}
}

func main() {
	log.SetOutput(io.Discard)
	email := flag.String("email", "", "An email to get better rate limits from Crossref API")
	nWorker := flag.Int("n", 1, "Number of workers for concurrent processing")
	verbose := flag.Bool("v", false, "Produce verbose output")
	checkURLs := flag.Bool("urls", false, "Check URLs in bibliography entries")
	verify := flag.Bool("verify", false, "Verify title, author, and year against Crossref metadata")
	maxRetries := flag.Int("retry", 3, "Max retries when fetching DOI data on rate limit (429)") //nolint: mnd
	flag.Parse()

	if *verify && *email == "" {
		fmt.Fprintln(os.Stderr, "hint: running -verify without -email may hit Crossref rate limits; consider adding -email <your@email.com>")
	}

	config := NewConfig(*email, *verbose, *checkURLs, *verify, *nWorker, *maxRetries)

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
	f, err := os.Open(file) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open %s: %w", file, err)
	}
	defer func() { _ = f.Close() }()

	jobs, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}

	spinner := &Spinner{}
	spinner.Start(len(jobs))

	reporter := &Reporter{w: os.Stdout, verbose: config.verbose}
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
		fmt.Fprintln(os.Stderr, "bibcheck [-email string] [-n int] [-v] [-urls] [-verify] [-retry int] <file.bib|file.yaml>")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}
	return flag.Arg(0)
}
