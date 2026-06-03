set shell := ["bash", "-euo", "pipefail", "-c"]
set tempdir := "/tmp"

binary := "helm-schema"
main_package := "./cmd/helm-schema"

# List available recipes.
default:
    @just --list

# Format Go source files.
fmt:
    go fmt ./...

# Tidy Go module dependencies.
tidy:
    go mod tidy

# Update all Go module dependencies to latest compatible versions.
update-deps:
    go get -u ./...
    go mod tidy

# Update Go module dependencies to latest compatible patch versions.
update-deps-patch:
    go get -u=patch ./...
    go mod tidy

# Run all Go unit tests.
test:
    go test ./...

# Run tests for one package. Example: just test-package ./pkg/schema
test-package package:
    go test {{package}}

# Run one named Go test. Example: just test-one ./pkg/schema TestTopoSort
test-one package name:
    go test {{package}} -run {{name}}

# Build the local CLI binary.
build:
    go build -o {{binary}} {{main_package}}

# Build the binary used by integration tests.
build-integration:
    go build -o tests/{{binary}} {{main_package}}

# Run integration tests against tests/helm-schema.
integration-test: build-integration
    #!/usr/bin/env bash
    set -euo pipefail

    cleanup() {
      rm -f tests/charts/*_generated.schema.json
      rm -f tests/charts/test_repo_example.yaml
      rm -f tests/charts/test_repo_example_expected.schema.json
    }
    trap cleanup EXIT

    cd tests && ./run.sh

# Run formatting, module tidy, unit tests, and integration tests.
check: fmt tidy test integration-test

# Validate GoReleaser configuration.
release-check:
    goreleaser check

# Build release artifacts locally without publishing.
snapshot:
    goreleaser release --snapshot --clean

# Generate local release notes for the latest tag range.
release-notes:
    git-cliff --latest --strip header --verbose --output RELEASE_NOTES.md

# Update release version files. Example: just set-version 0.24.0
set-version version:
    #!/usr/bin/env bash
    set -euo pipefail

    if [[ ! "{{version}}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
      echo "Version must be bare SemVer, for example: 0.24.0" >&2
      exit 1
    fi

    sed -i -E 's/^var version string = ".*"$/var version string = "{{version}}"/' cmd/helm-schema/version.go
    sed -i -E 's/^version: ".*"$/version: "{{version}}"/' plugin.yaml

    grep -qx 'var version string = "{{version}}"' cmd/helm-schema/version.go
    grep -qx 'version: "{{version}}"' plugin.yaml

# Create and push a release tag. Example: just release 0.24.0
release version:
    #!/usr/bin/env bash
    set -euo pipefail

    if [[ ! "{{version}}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
      echo "Release version must be bare SemVer, for example: 0.24.0" >&2
      exit 1
    fi

    if [[ -n "$(git status --porcelain)" ]]; then
      echo "Working tree must be clean before creating a release tag." >&2
      exit 1
    fi

    git fetch --tags

    if git rev-parse --verify --quiet "refs/tags/{{version}}" >/dev/null; then
      echo "Tag already exists: {{version}}" >&2
      exit 1
    fi

    just set-version "{{version}}"
    just check

    unexpected_changes="$(git status --porcelain | awk '{print $2}' | grep -Ev '^(cmd/helm-schema/version.go|plugin.yaml)$' || true)"
    if [[ -n "$unexpected_changes" ]]; then
      echo "Release checks changed files outside the version bump:" >&2
      echo "$unexpected_changes" >&2
      exit 1
    fi

    if ! git diff --quiet -- cmd/helm-schema/version.go plugin.yaml; then
      git add cmd/helm-schema/version.go plugin.yaml
      git commit -m "chore: release {{version}}"
    fi

    git tag -a "{{version}}" -m "Release {{version}}"
    git push origin HEAD
    git push origin "{{version}}"

# Remove local build artifacts.
clean:
    rm -rf {{binary}} tests/{{binary}} dist RELEASE_NOTES.md
    rm -f tests/charts/*_generated.schema.json
    rm -f tests/charts/test_repo_example.yaml
    rm -f tests/charts/test_repo_example_expected.schema.json

alias b := build
alias c := check
alias i := integration-test
alias s := snapshot
alias t := test
