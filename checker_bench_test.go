package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	benchDelay   = 50 * time.Millisecond
	benchEntries = 50
)

var benchWorkers = []int{1, 2, 4, 8}

func makeBenchJobs(n int) []job {
	jobs := make([]job, n)
	for i := range n {
		jobs[i] = job{
			citeName:  fmt.Sprintf("entry%d", i),
			doi:       fmt.Sprintf("10.1000/bench.%d", i),
			entryType: "article",
			title:     "Benchmark Title",
			author:    "Smith, John",
			year:      "2023",
		}
	}
	return jobs
}

func BenchmarkProcessJobs_DOIOnly(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(benchDelay)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	jobs := makeBenchJobs(benchEntries)

	for _, n := range benchWorkers {
		b.Run(fmt.Sprintf("workers=%d", n), func(b *testing.B) {
			cfg := &Config{
				client:          srv.Client(),
				crossrefBaseURL: srv.URL + "/",
				nWorker:         n,
				maxRetries:      1,
			}
			for b.Loop() {
				start := time.Now()
				for range processJobs(cfg, jobs) {
				}
				b.ReportMetric(float64(benchEntries)/time.Since(start).Seconds(), "entries/s")
			}
		})
	}
}

func BenchmarkProcessJobs_WithVerify(b *testing.B) {
	cr := crossrefResponse{
		Message: crossrefMessage{
			Title:     []string{"Benchmark Title"},
			Author:    []crossrefAuthor{{Family: "Smith", Given: "John"}},
			Published: crossrefDate{DateParts: [][]int{{2023}}},
		},
	}
	body, _ := json.Marshal(cr)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(benchDelay)
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	jobs := makeBenchJobs(benchEntries)

	for _, n := range benchWorkers {
		b.Run(fmt.Sprintf("workers=%d", n), func(b *testing.B) {
			cfg := &Config{
				client:          srv.Client(),
				crossrefBaseURL: srv.URL + "/",
				nWorker:         n,
				maxRetries:      1,
				verify:          true,
			}
			for b.Loop() {
				start := time.Now()
				for range processJobs(cfg, jobs) {
				}
				b.ReportMetric(float64(benchEntries)/time.Since(start).Seconds(), "entries/s")
			}
		})
	}
}
