# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`helm-schema` is a Go tool that generates JSON Schema (Draft 7) files for Helm chart values. It traverses directories to find `Chart.yaml` files, reads associated `values.yaml` files, parses special `@schema` annotations in comments, and generates `values.schema.json` files that enable IDE auto-completion and validation for Helm values.

## Build and Test Commands

### Build
```bash
# Build the binary
go build -o helm-schema ./cmd/helm-schema

# Build with goreleaser (for releases)
goreleaser release --snapshot --clean
```

### Test
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/schema
go test ./pkg/chart

# Run a specific test
go test ./pkg/schema -run TestTopoSort

# Run tests with verbose output
go test -v ./...

# Integration tests (requires helm-schema binary in tests/)
cd tests && ./run.sh
```

### Linting and Formatting
```bash
# Format code
go fmt ./...

# Tidy dependencies
go mod tidy
```

## Code Architecture

### High-Level Flow

1. **Chart Discovery** (`pkg/chart/searching/`): Recursively searches for `Chart.yaml` files starting from a root directory. Also extracts `.tgz` chart archives if found.

2. **Parallel Processing** (`cmd/helm-schema/main.go`): Uses a worker pool (2x CPU cores) to process charts concurrently. Each worker receives chart paths via a channel.

3. **Schema Generation** (`pkg/schema/worker.go`): For each chart, reads the `values.yaml` file and parses YAML with annotations to build a JSON Schema object.

4. **Annotation Parsing** (`pkg/schema/schema.go`): Parses `# @schema` and `# @schema.root` comment blocks to extract JSON Schema properties (type, description, enum, pattern, etc.).

5. **Dependency Resolution** (`cmd/helm-schema/main.go`): After all charts are processed, topologically sorts them by dependencies and merges dependency schemas into parent charts. Library charts (type: library) have their properties merged at the top level.

6. **Output** (`cmd/helm-schema/main.go`): Writes `values.schema.json` files to each chart directory.

### Key Components

#### Schema Parsing (`pkg/schema/schema.go`)
- **`ParseValues()`**: Main entry point that parses a values.yaml file and returns a Schema
- **`parseYamlNode()`**: Recursively traverses YAML nodes, extracting schema annotations and inferring types
- **Annotation blocks**: Comments between `# @schema` markers are parsed as YAML to extract JSON Schema properties
- **Root annotations**: Comments between `# @schema.root` markers apply to the root schema object itself
- **Type inference**: If no type is specified, the tool infers it from YAML tags (!!str, !!int, !!bool, etc.)

#### Worker Pattern (`pkg/schema/worker.go`)
- Workers pull chart paths from a channel and process them independently
- Each worker:
  1. Reads Chart.yaml
  2. Finds values.yaml (tries multiple filenames from config)
  3. Parses values into a Schema
  4. Sends Result to results channel

#### Dependency Graph (`pkg/schema/toposort.go`)
- **TopoSort()**: Uses DFS-based topological sorting to ensure dependencies are processed before dependents
- Detects circular dependencies and can either fail or warn based on `allowCircular` flag
- Returns charts in dependency order (dependencies first, parents last)

#### Chart Models (`pkg/chart/chart.go`)
- **ChartFile**: Represents Chart.yaml structure
- **Dependency**: Represents a chart dependency with name, version, alias, condition

#### Schema Merging (in `main.go`)
- Regular dependencies: Nested under dependency name (or alias) in parent schema
- Library charts: Properties merged directly into parent schema at top level
- Conditional dependencies: If a dependency has a `condition` field, the corresponding boolean property is auto-created in the dependency's schema
- Skip validation flag (`-m`): Can disable strict validation for dependencies by setting `additionalProperties: true`

### Important Patterns

1. **BoolOrArrayOfString**: The `required` field in JSON Schema can be either a boolean or an array of strings. This custom type handles both cases during marshaling/unmarshaling.

2. **SchemaOrBool**: Some JSON Schema fields like `additionalProperties` can be either a boolean or a Schema object. This is represented as `interface{}`.

3. **Annotation Comments**: The tool looks for comments in specific formats:
   - `# @schema` / `# @schema` blocks for field-level annotations
   - `# @schema.root` / `# @schema.root` blocks for root-level annotations
   - Comments outside these blocks become descriptions (unless `description` is explicitly set)

4. **Helm-docs compatibility**: With `-p` flag, parses `-- helm-docs description` and `@default` annotations from helm-docs format.

## Testing Strategy

- **Unit tests**: Each package has `*_test.go` files testing individual functions
- **Integration tests**: `tests/run.sh` compares generated schemas against expected outputs
- **Test files**: `tests/test_*.yaml` are input values files, `tests/test_*_expected.schema.json` are expected outputs

## Plugin Verification (Helm v4)

The project implements Helm v4 plugin verification through GPG signing:

### Signing Infrastructure

1. **sign-plugin.sh**: Script that creates `.prov` (provenance) files for plugin tarballs
   - Takes version, tarball path, and GPG key as arguments
   - Creates a signed provenance file containing metadata and SHA256 hash
   - Uses GPG to sign the provenance

2. **GitHub Actions Workflow**: `.github/workflows/release.yml`
   - Imports GPG private key from secrets (`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`)
   - Runs goreleaser to build and package binaries
   - Signs all `.tar.gz` files with `sign-plugin.sh`
   - Uploads `.prov` files to GitHub releases

3. **GoReleaser Config**: `.goreleaser.yaml`
   - Archives include plugin files: `plugin.yaml`, `install-binary.sh`, `README.md`, `LICENSE`
   - Configured to sign checksums with GPG

### Setup for Maintainers

- See `.github/SETUP_SIGNING.md` for initial GPG key setup
- Public key should be in `signing-key.asc` (currently a template)
- Key details must be updated in `VERIFICATION.md`

### Verification Process

Users can verify plugins with:
```bash
helm plugin install <tarball> --verify
helm plugin verify schema
```

## Supported JSON Schema Draft 7 Keywords

The Schema struct (`pkg/schema/schema.go`) supports the following JSON Schema Draft 7 keywords:

### Core Keywords
- `$schema`, `$id`, `$ref`, `$comment`
- `type` (single type or array of types)
- `title`, `description`
- `default`, `examples`
- `definitions` (also supports `$defs` from Draft 2019-09+ - automatically converted to `definitions`)

### Validation Keywords

#### Numeric (number, integer)
- `minimum`, `maximum` (float64 - supports decimal values like `1.5`)
- `exclusiveMinimum`, `exclusiveMaximum` (float64)
- `multipleOf` (float64 - supports `0.1`, `0.01`, etc.)

#### String
- `minLength`, `maxLength`
- `pattern` (regex pattern)
- `format` (date-time, email, uri, ipv4, ipv6, uuid, etc.)
- `contentEncoding`, `contentMediaType`

#### Array
- `items` (single schema or handled via anyOf for arrays)
- `additionalItems` (boolean or schema)
- `minItems`, `maxItems`
- `uniqueItems`
- `contains`

#### Object
- `properties`, `patternProperties`
- `additionalProperties` (boolean or schema)
- `required` (boolean or array of strings)
- `minProperties`, `maxProperties`
- `propertyNames`
- `dependencies`

### Composition Keywords
- `allOf`, `anyOf`, `oneOf`, `not`
- `if`, `then`, `else`

### Annotation Keywords
- `deprecated`, `readOnly`, `writeOnly`
- `enum`, `const`

### Custom Annotations
- Any key prefixed with `x-` is treated as a custom annotation

## Validation Behavior

The schema validation (`Validate()` method) performs type-specific constraint checks:

- Numeric constraints (`minimum`, `maximum`, etc.) require `type: number` or `type: integer`
- String constraints (`minLength`, `maxLength`, `pattern`, `format`, `contentEncoding`) require `type: string`
- Array constraints (`items`, `minItems`, `maxItems`, `contains`, `additionalItems`) require `type: array`
- Object constraints (`minProperties`, `maxProperties`, `propertyNames`, `additionalProperties`) require `type: object`

Some keywords like `uniqueItems` are accepted on any type per the JSON Schema spec (keywords are ignored if the type doesn't match).

## Common Gotchas

1. **Draft 7 limitation**: The tool uses JSON Schema Draft 7 because Helm's validation library only supports that version. When referencing external schemas that use `$defs` (Draft 2019-09+), they are automatically converted to `definitions` and `$ref` paths are rewritten from `#/$defs/` to `#/definitions/`.

2. **Root annotations placement**: `@schema.root` blocks must be at the top of values.yaml with no blank lines after (unless using `-s` flag).

3. **Dependency schema immutability**: You cannot modify a dependency's schema using annotations in the parent chart's values.yaml. The schema comes from the dependency's own values.yaml.

4. **Library chart merging**: When a library chart property name conflicts with a parent property, the parent takes precedence (with a warning logged).

5. **Comment parsing**: By default, descriptions are cut at the first empty line in comments. Use `-s` to keep full comments.

6. **Plugin signing**: Signing only works if GPG secrets are configured in GitHub. Missing secrets cause signing steps to be skipped gracefully.
