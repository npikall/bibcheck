package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newReporter(t *testing.T, verbose bool) (*Reporter, *bytes.Buffer) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	buf := &bytes.Buffer{}
	return &Reporter{w: buf, verbose: verbose}, buf
}

// --- Collect ---

func TestReporterCollect(t *testing.T) {
	r, _ := newReporter(t, false)
	r.Collect(EntryResult{CiteName: "key1", DOI: "10.1/a"})
	r.Collect(EntryResult{CiteName: "key2", DOI: "10.1/b"})
	assert.Len(t, r.results, 2)
}

// --- Print: verbose OK entry ---

func TestReporterPrint_VerboseOK(t *testing.T) {
	r, buf := newReporter(t, true)
	r.Collect(EntryResult{CiteName: "key1", DOI: "10.1/a", Issues: nil})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, " OK ")
	assert.Contains(t, out, "key1")
	assert.Contains(t, out, "10.1/a")
}

func TestReporterPrint_NonVerboseOKSilent(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{CiteName: "key1", DOI: "10.1/a", Issues: nil})
	r.Print()
	out := buf.String()
	assert.NotContains(t, out, "key1")
}

// --- printIssue: DOI issues ---

func TestReporterPrint_NoDOI(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		Issues:   []Issue{{Kind: IssueNoDOI}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "key1")
	assert.Contains(t, out, "no DOI field")
	assert.Contains(t, out, "WARN")
}

func TestReporterPrint_InvalidDOI(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		Issues:   []Issue{{Kind: IssueInvalidDOI, Message: "empty after normalization"}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "invalid DOI")
	assert.Contains(t, out, "empty after normalization")
	assert.Contains(t, out, "ERRO")
}

func TestReporterPrint_DOINotFound(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		DOI:      "10.1/bad",
		Issues:   []Issue{{Kind: IssueDOINotFound, Message: "HTTP 404"}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "DOI not found")
	assert.Contains(t, out, "HTTP 404")
	assert.Contains(t, out, "10.1/bad")
}

// --- printIssue: URL issues ---

func TestReporterPrint_URLDead(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		URL:      "https://dead.example.com",
		Issues:   []Issue{{Kind: IssueURLDead, Message: "HTTP 404"}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "URL dead")
	assert.Contains(t, out, "https://dead.example.com")
	assert.Contains(t, out, "ERRO")
}

func TestReporterPrint_URLForbidden(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		URL:      "https://bot.example.com",
		Issues:   []Issue{{Kind: IssueURLForbidden, Message: "HTTP 403"}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "URL unverifiable")
	assert.Contains(t, out, "https://bot.example.com")
	assert.Contains(t, out, "WARN")
}

func TestReporterPrint_URLIssueNoMessage(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		URL:      "https://example.com",
		Issues:   []Issue{{Kind: IssueURLDead}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "url: https://example.com")
}

// --- printIssue: Diff ---

func TestReporterPrint_Diff(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "key1",
		DOI:      "10.1/x",
		Issues: []Issue{{
			Kind:    IssueDiff,
			Message: "title mismatch",
			Detail:  `         local:    "local title"\n         crossref: "remote title"`,
		}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "title mismatch")
	assert.Contains(t, out, "DIFF")
	assert.Contains(t, out, "10.1/x")
}

// --- printSummary ---

func TestReporterSummary_AllOK(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{CiteName: "a", Issues: nil})
	r.Collect(EntryResult{CiteName: "b", Issues: nil})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "Checked 2 entries")
	assert.Contains(t, out, "2 OK")
	assert.Contains(t, out, "0 warning(s)")
	assert.Contains(t, out, "0 error(s)")
	assert.NotContains(t, out, "diff(s)")
}

func TestReporterSummary_MixedIssues(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{CiteName: "a", Issues: []Issue{{Kind: IssueNoDOI}}})
	r.Collect(EntryResult{CiteName: "b", Issues: []Issue{{Kind: IssueDOINotFound, Message: "HTTP 404"}}})
	r.Collect(EntryResult{CiteName: "c", Issues: nil})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "Checked 3 entries")
	assert.Contains(t, out, "1 OK")
	assert.Contains(t, out, "1 warning(s)")
	assert.Contains(t, out, "1 error(s)")
}

func TestReporterSummary_WithDiffs(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "a",
		Issues:   []Issue{{Kind: IssueDiff, Message: "title mismatch"}},
	})
	r.Print()
	out := buf.String()
	assert.Contains(t, out, "1 diff(s)")
}

func TestReporterSummary_MultipleIssuesSameEntry(t *testing.T) {
	r, buf := newReporter(t, false)
	r.Collect(EntryResult{
		CiteName: "a",
		Issues: []Issue{
			{Kind: IssueNoDOI},
			{Kind: IssueURLDead, Message: "HTTP 404"},
		},
	})
	r.Print()
	out := buf.String()
	// one entry with issues → 0 OK
	assert.Contains(t, out, "0 OK")
	assert.True(t, strings.Contains(out, "1 warning(s)") || strings.Contains(out, "1 error(s)"))
}
