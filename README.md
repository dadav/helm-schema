# helm-schema

<p>
  <img src="images/logo.png" width="400" />
  <br />
  <a href="https://github.com/dadav/helm-schema/releases"><img src="https://img.shields.io/github/release/dadav/helm-schemas.svg" alt="Latest Release"></a>
  <a href="https://pkg.go.dev/github.com/dadav/helm-schemas?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="Go Docs"></a>
  <a href="https://github.com/dadav/helm-schema/actions"><img src="https://github.com/dadav/helm-schema/workflows/build/badge.svg" alt="Build Status"></a>
</p>


This tool tries to help you to easily create some nice jsonschema for your helm chart.

By default it will traverse the current directory and look for `Chart.yaml` files.
For every file, helm-schema will try to find one of the given value filenames.
The first files found will be read and a jsonschema will be created.
For every dependency defined in the Chart.yaml file, a reference to the dependencies jsonschema
will be created.

## Installation

```bash
go install github.com/dadav/helm-schema/cmd/helm-schema@latest
```

## Usage

### Pre-commit hook

If you want to automatically generate a new `values.schema.json` if you change the `values.yaml`
file, you can do the following:

```bash
pre-commit install
pre-commit install-hooks
```

### Running the binary directly

You can also just run the binary yourself:

```bash
helm-schema
```

### Options

The binary has the following options:

```bash
Flags:
  -c, --chart-search-root string   directory to search recursively within for charts (default ".")
  -d, --dry-run                    don't actually create files just print to stdout passed
  -h, --help                       help for helm-schema
  -s, --keep-full-comment          If this flag is used, comment won't be cut off if two newlines are found.
  -l, --log-level string           Level of logs that should printed, one of (panic, fatal, error, warning, info, debug, trace) (default "info")
  -n, --no-dependencies            don't analyze dependencies
  -o, --output-file string         jsonschema file path relative to each chart directory to which jsonschema will be written (default "values.schema.json")
  -f, --value-files strings        filenames to check for chart values (default [values.yaml])
  -v, --version                    version for helm-schema
```

## Annotations

Each annotation must start with `# @schema`.
It must be written in front of the key you want to annotate.

Valid options are:

**type**
Defines the [jsonschema-type](https://json-schema.org/understanding-json-schema/reference/type.html) of the object
Possible values are:
> object, array, string, number, integer, null

**title**
Defines the [title field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=title) of the object
Defaults to the key itself.

**description**
Defines the [description field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=description) of the object
Defaults to the comment which has no `# @schema` prefix.

**items**
Takes a string of possible array value types separated by `|`.
Implies `type=array`.
Example: Array can contain string or null items: `items=string|null`

**pattern**
Regex pattern to test the value.
Implies `type=string`.

**format**
The [format keyword](https://json-schema.org/understanding-json-schema/reference/string.html#format) allows for basic semantic identification of certain kinds of string values.
Implies `type=string`.

**required**
Adds the key to the required items. Takes boolean values.

**examples**
Some examples your can provide for the enduser. They must be separated by `|`

**min**
Minimum value.
Can't be used with `xmin`.
Must be smaller than `max` or `xmax` (if used).

**xmin**
Exclusive minimum.
Can't be used with `min`.
Must be smaller than `max` or `xmax` (if used).

**max**
Maximum value.
Can't be used with `xmax`.
Must be bigger than `min` or `xmin` (if used).

**xmax**
Exclusive maximum value.
Can't be used with `max`.
Must be bigger than `min` or `xmin` (if used).

**multiple**
Value, the yaml-value must be a multiple of.
For example: If you set this to 10, allowed values would be 0, 10, 20, 30....

**const**
Single allowed value.

**enum**
Multiple allowed values.

## Example

This values.yaml:

```yaml
---
# yaml-language-server: $schema=values.schema.json

# This text will be used as description.
# @schema type=integer min=0 required=true
foo: 1

# If your use multiple annotations, they are treated as an alternative (one of these must match).
# @schema type=null
# @schema pattern=^foo
bar:

# You can use backslashes to make it more readable.
# If you don't use `type`, the type will be guessed from the
# default value.
# @schema \
#   title=Some title \
#   description=Some description \
#   required=true 
baz: foo
```

Will result in this jsonschema:

```json
{
  "$schema": "http://json-schema.org/schema#",
  "properties": {
    "bar": {
      "oneOf": [
        {
          "default": "",
          "description": "If your use multiple annotations, they are treated as an alternative (one of these must match).",
          "title": "bar",
          "type": "null"
        },
        {
          "default": "",
          "description": "If your use multiple annotations, they are treated as an alternative (one of these must match).",
          "pattern": "^foo",
          "title": "bar",
          "type": "string"
        }
      ]
    },
    "baz": {
      "default": "foo",
      "description": "Some description",
      "title": "Some title"
    },
    "foo": {
      "default": "1",
      "description": "This text will be used as description.",
      "minimum": 0,
      "title": "foo",
      "type": "integer"
    }
  },
  "required": [
    "foo",
    "baz"
  ],
  "type": "object"
}
```
