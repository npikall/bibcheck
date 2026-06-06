package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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

type hayagrivaEntry struct {
	URL          hayagrivaURL `yaml:"url"`
	SerialNumber struct {
		DOI string `yaml:"doi"`
	} `yaml:"serial-number"`
}

type YAMLParser struct{}

func (p *YAMLParser) Parse(file string) (_ []job, err error) {
	f, err := os.Open(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var entries map[string]hayagrivaEntry
	if err := yaml.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	jobs := make([]job, 0, len(entries))
	for key, entry := range entries {
		j := job{citeName: key}

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
