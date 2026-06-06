package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

var errDateNotScalar = errors.New("date: expected scalar node")

// hayagrivaURL handles both scalar and object forms of the url field.
type hayagrivaURL struct {
	Value string
}

func (u *hayagrivaURL) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		u.Value = value.Value
		return nil
	}
	var obj struct {
		Value string `yaml:"value"`
	}
	if err := value.Decode(&obj); err != nil {
		return fmt.Errorf("decode yaml value: %w", err)
	}
	u.Value = obj.Value
	return nil
}

// hayagrivaTitle handles both scalar and object forms of the title field.
type hayagrivaTitle struct {
	Value string
}

func (t *hayagrivaTitle) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Value = value.Value
		return nil
	}
	var obj struct {
		Value string `yaml:"value"`
	}
	if err := value.Decode(&obj); err != nil {
		return fmt.Errorf("decode yaml title: %w", err)
	}
	t.Value = obj.Value
	return nil
}

// hayagrivaAuthor handles both scalar and sequence forms of the author field.
type hayagrivaAuthor struct {
	Names []string
}

func (a *hayagrivaAuthor) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		a.Names = []string{value.Value}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var names []string
		if err := value.Decode(&names); err != nil {
			return fmt.Errorf("decode yaml author: %w", err)
		}
		a.Names = names
		return nil
	}
	return nil
}

// hayagrivaDate handles both integer and string forms of the date field, extracting the year.
type hayagrivaDate struct {
	Year string
}

func (d *hayagrivaDate) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errDateNotScalar
	}
	year := value.Value
	if len(year) > 4 { //nolint:mnd
		year = year[:4]
	}
	d.Year = year
	return nil
}

type hayagrivaEntry struct {
	Type         string          `yaml:"type"`
	Title        hayagrivaTitle  `yaml:"title"`
	Author       hayagrivaAuthor `yaml:"author"`
	Date         hayagrivaDate   `yaml:"date"`
	URL          hayagrivaURL    `yaml:"url"`
	SerialNumber struct {
		DOI string `yaml:"doi"`
	} `yaml:"serial-number"`
}

type YAMLParser struct{}

func (p *YAMLParser) Parse(r io.Reader) (_ []job, err error) {
	var entries map[string]hayagrivaEntry
	if err := yaml.NewDecoder(r).Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	jobs := make([]job, 0, len(entries))
	for key, entry := range entries {
		j := job{
			citeName:  key,
			entryType: entry.Type,
			title:     entry.Title.Value,
			author:    strings.Join(entry.Author.Names, " and "),
			year:      entry.Date.Year,
		}

		doi := normalizeDOI(entry.SerialNumber.DOI)
		if doi == "" {
			j.localIssues = append(j.localIssues, Issue{Kind: IssueNoDOI})
		} else {
			j.doi = doi
		}

		if url := strings.TrimSpace(entry.URL.Value); url != "" {
			j.url = url
		}

		jobs = append(jobs, j)
	}
	return jobs, nil
}
