# Run commands or execute tasks related to the repository development
_default:
    @just --list

alias b := build
alias fmt := format


BIN_NAME := "bibcheck"
LDFLAGS := "-s -w"

# build the binary
build *args:
    go build -ldflags="{{ LDFLAGS }}" -o {{ BIN_NAME }} {{ args }}

# install the binary locally
install *args:
    go install -ldflags="{{ LDFLAGS }}" {{ args }}

# run the test suite
test *args:
    go test ./... {{ args }}

# run benchmarks
bench filter=".":
    go test -bench={{ filter }} -benchtime=3x -timeout=600s

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
