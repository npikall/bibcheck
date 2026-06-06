# Run commands or execute tasks related to the repository development
_default:
    @just --list

alias b := build
alias fmt := format


BIN_NAME := "bibcheck"
LDFLAGS := "-s -w"

# build the binary
build:
    go build -ldflags="{{ LDFLAGS }}" -o {{ BIN_NAME }}

# install the binary locally
install:
    go install -ldflags="{{ LDFLAGS }}"

# run the test suite
test:
    go test ./...

# run the go formatter
format:
    go fmt ./...
    golangci-lint fmt

# run the linter
lint:
    golangci-lint run --fix

_ensure_clean:
    @git diff --quiet
    @git diff --cached --quiet

_commit_and_tag version:
    git add CHANGELOG.md
    git commit -m "chore(release): bump version to {{ version }}"
    git tag -a "v{{ version }}"

# make a new release (e.g. semver=0.1.2)
release semver:
    @just _ensure_clean
    @just changelog --tag {{ semver }}
    @just _commit_and_tag {{ semver }}
    @echo "{{ GREEN }}Release complete. Run 'git push && git push --tags'.{{ NORMAL }}"
