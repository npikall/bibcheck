# Bibcheck

CLI tool that validates DOIs in a BibTeX file by checking them against the
[Crossref API](https://api.crossref.org).

## Usage

```console
bibcheck [-email string] [-n int] [-v] <file.bib>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-email` | — | Contact email for Crossref polite pool (better rate limits) |
| `-n` | `1` | Number of concurrent workers (capped at CPU count) |
| `-v` | `false` | Verbose output |

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
```
