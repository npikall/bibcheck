# Bibcheck

CLI tool that validates bibliography entries (`.bib` or `.yaml`) by checking DOIs against the
[Crossref API](https://api.crossref.org) and optionally verifying URLs.

## Usage

```console
bibcheck [-email string] [-n int] [-v] [-urls] [-verify] [-retry int] <file.bib|file.yaml>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-email` | — | Contact email for Crossref polite pool (better rate limits) |
| `-n` | `1` | Number of concurrent workers (capped at CPU count) |
| `-v` | `false` | Verbose output |
| `-urls` | `true` | Check URLs in bibliography entries |
| `-verify` | `false` | Verify title, author, and year against Crossref metadata |
| `-retry` | `3` | Max retries when fetching DOI data on rate limit (429) |

## Install

```sh
go install github.com/npikall/bibcheck@latest
```

Or build from source:

```sh
git clone https://github.com/npikall/bibcheck
cd bibcheck
go build -o bibcheck .
```

## Example

```sh
bibcheck -email you@example.com -n 4 refs.bib
bibcheck -email you@example.com -verify -n 4 refs.bib
bibcheck -urls=false refs.yaml
```
