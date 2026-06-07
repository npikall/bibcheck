package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const yamlWithDOI = `
smith2023:
  type: article
  title: A Great Paper
  author:
    - Smith, John
  date: 2023
  serial-number:
    doi: 10.1000/xyz123
`

const yamlNoDOI = `
doe2022:
  type: article
  title: Another Paper
  author:
    - Doe, Jane
  date: 2022
`

const yamlObjectTitle = `
obj2023:
  type: article
  title:
    value: Object Title
  author:
    - Author, A
  date: 2023
  serial-number:
    doi: 10.1/obj
`

const yamlObjectURL = `
url2023:
  type: web
  title: URL Entry
  date: 2023
  url:
    value: https://example.com
`

const yamlScalarURL = `
url2024:
  type: web
  title: URL Entry 2
  date: 2024
  url: https://example.org
`

const yamlFullDate = `
full2023:
  type: article
  title: Full Date
  author:
    - Author, A
  date: 2023-06-15
  serial-number:
    doi: 10.1/full
`

const yamlMultipleAuthors = `
multi2023:
  type: article
  title: Multi Author
  author:
    - Smith, John
    - Doe, Jane
  date: 2023
  serial-number:
    doi: 10.1/multi
`

const yamlScalarAuthor = `
scalar2023:
  type: article
  title: Scalar Author
  author: Smith, John
  date: 2023
  serial-number:
    doi: 10.1/scalar
`

const yamlDOIWithPrefix = `
prefix2023:
  type: article
  title: Prefixed DOI
  date: 2023
  serial-number:
    doi: https://doi.org/10.1000/prefix
`

func TestYAMLParser_WithDOI(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlWithDOI))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Equal(t, "smith2023", j.citeName)
	assert.Equal(t, "article", j.entryType)
	assert.Equal(t, "10.1000/xyz123", j.doi)
	assert.Equal(t, "A Great Paper", j.title)
	assert.Equal(t, "2023", j.year)
	assert.Empty(t, j.localIssues)
}

func TestYAMLParser_NoDOI(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlNoDOI))
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	j := jobs[0]
	assert.Empty(t, j.doi)
	require.Len(t, j.localIssues, 1)
	assert.Equal(t, IssueNoDOI, j.localIssues[0].Kind)
}

func TestYAMLParser_ObjectTitle(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlObjectTitle))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "Object Title", jobs[0].title)
}

func TestYAMLParser_ObjectURL(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlObjectURL))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "https://example.com", jobs[0].url)
}

func TestYAMLParser_ScalarURL(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlScalarURL))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "https://example.org", jobs[0].url)
}

func TestYAMLParser_FullDateExtractsYear(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlFullDate))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "2023", jobs[0].year)
}

func TestYAMLParser_MultipleAuthors(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlMultipleAuthors))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "Smith, John and Doe, Jane", jobs[0].author)
}

func TestYAMLParser_ScalarAuthor(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlScalarAuthor))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "Smith, John", jobs[0].author)
}

func TestYAMLParser_DOINormalization(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	jobs, err := p.Parse(strings.NewReader(yamlDOIWithPrefix))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "10.1000/prefix", jobs[0].doi)
}

func TestYAMLParser_InvalidInput(t *testing.T) {
	t.Parallel()
	p := &YAMLParser{}
	_, err := p.Parse(strings.NewReader(":\t\tinvalid: yaml: {{"))
	assert.Error(t, err)
}

// --- Custom unmarshalers ---

func TestHayagrivaURLScalar(t *testing.T) {
	t.Parallel()
	var u hayagrivaURL
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "https://example.com"}
	require.NoError(t, u.UnmarshalYAML(node))
	assert.Equal(t, "https://example.com", u.Value)
}

func TestHayagrivaURLObject(t *testing.T) {
	t.Parallel()
	var u hayagrivaURL
	node := &yaml.Node{}
	require.NoError(t, yaml.Unmarshal([]byte("value: https://example.com"), node))
	require.NoError(t, u.UnmarshalYAML(node.Content[0]))
	assert.Equal(t, "https://example.com", u.Value)
}

func TestHayagrivaTitleScalar(t *testing.T) {
	t.Parallel()
	var ti hayagrivaTitle
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "My Title"}
	require.NoError(t, ti.UnmarshalYAML(node))
	assert.Equal(t, "My Title", ti.Value)
}

func TestHayagrivaTitleObject(t *testing.T) {
	t.Parallel()
	var ti hayagrivaTitle
	node := &yaml.Node{}
	require.NoError(t, yaml.Unmarshal([]byte("value: My Title"), node))
	require.NoError(t, ti.UnmarshalYAML(node.Content[0]))
	assert.Equal(t, "My Title", ti.Value)
}

func TestHayagrivaAuthorScalar(t *testing.T) {
	t.Parallel()
	var a hayagrivaAuthor
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "Smith, John"}
	require.NoError(t, a.UnmarshalYAML(node))
	assert.Equal(t, []string{"Smith, John"}, a.Names)
}

func TestHayagrivaAuthorSequence(t *testing.T) {
	t.Parallel()
	var a hayagrivaAuthor
	node := &yaml.Node{}
	require.NoError(t, yaml.Unmarshal([]byte("- Smith, John\n- Doe, Jane"), node))
	require.NoError(t, a.UnmarshalYAML(node.Content[0]))
	assert.Equal(t, []string{"Smith, John", "Doe, Jane"}, a.Names)
}

func TestHayagrivaDateScalar(t *testing.T) {
	t.Parallel()
	var d hayagrivaDate
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "2023"}
	require.NoError(t, d.UnmarshalYAML(node))
	assert.Equal(t, "2023", d.Year)
}

func TestHayagrivaDateFullString(t *testing.T) {
	t.Parallel()
	var d hayagrivaDate
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "2023-06-15"}
	require.NoError(t, d.UnmarshalYAML(node))
	assert.Equal(t, "2023", d.Year)
}

func TestHayagrivaDateNonScalarError(t *testing.T) {
	t.Parallel()
	var d hayagrivaDate
	node := &yaml.Node{Kind: yaml.MappingNode}
	err := d.UnmarshalYAML(node)
	assert.ErrorIs(t, err, errDateNotScalar)
}
