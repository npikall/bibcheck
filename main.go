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

	"github.com/nickng/bibtex"
)

type Config struct {
	client  *http.Client
	email   string
	verbose bool
	nWorker int
}

func NewConfig(email string, verbose bool, n int) *Config {
	return &Config{
		client:  &http.Client{Timeout: 5 * time.Second},
		email:   email,
		verbose: verbose,
		nWorker: n,
	}
}

func main() {
	log.SetOutput(io.Discard)
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
