package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bibWithDOI = `@article{smith2023,
  author = {Smith, John},
  title  = {A Great Paper},
  year   = {2023},
  doi    = {10.1000/xyz123},
}`

const bibNoDOI = `@article{doe2022,
  author = {Doe, Jane},
  title  = {Another Paper},
  year   = {2022},
}`

const bibEmptyDOI = `@article{empty2021,
  author = {Author, A},
  title  = {Empty DOI Paper},
  year   = {2021},
  doi    = {https://doi.org/},
}`

const bibWithURL = `@misc{web2020,
  title = {Some Website},
  url   = {https://example.com},
}`

const bibMultiple = `@article{a2023,
  author = {Alpha, A},
  title  = {First},
  year   = {2023},
  doi    = {10.1/a},
}
@book{b2022,
  author = {Beta, B},
  title  = {Second},
  year   = {2022},
  doi    = {10.1/b},
}`

func TestBibParser_WithDOI(t *testing.T) {
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bibWithDOI))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Equal(t, "smith2023", j.citeName)
	assert.Equal(t, "10.1000/xyz123", j.doi)
	assert.Equal(t, "article", j.entryType)
	assert.Equal(t, "A Great Paper", j.title)
	assert.Equal(t, "Smith, John", j.author)
	assert.Equal(t, "2023", j.year)
	assert.Empty(t, j.localIssues)
}

func TestBibParser_NoDOI(t *testing.T) {
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bibNoDOI))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Equal(t, "doe2022", j.citeName)
	assert.Empty(t, j.doi)
	require.Len(t, j.localIssues, 1)
	assert.Equal(t, IssueNoDOI, j.localIssues[0].Kind)
}

func TestBibParser_EmptyDOIAfterNormalization(t *testing.T) {
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bibEmptyDOI))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Empty(t, j.doi)
	require.Len(t, j.localIssues, 1)
	assert.Equal(t, IssueInvalidDOI, j.localIssues[0].Kind)
}

func TestBibParser_WithURL(t *testing.T) {
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bibWithURL))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Equal(t, "https://example.com", j.url)
}

func TestBibParser_Multiple(t *testing.T) {
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bibMultiple))
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}

func TestBibParser_InvalidInput(t *testing.T) {
	p := &BibParser{}
	_, err := p.Parse(strings.NewReader("@article{missing_brace"))
	assert.Error(t, err)
}

func TestBibParser_DOINormalization(t *testing.T) {
	const bib = `@article{norm2023,
  author = {Author, A},
  title  = {Title},
  year   = {2023},
  doi    = {https://doi.org/10.1000/norm},
}`
	p := &BibParser{}
	jobs, err := p.Parse(strings.NewReader(bib))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "10.1000/norm", jobs[0].doi)
}
