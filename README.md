# helm-schema

<p align="center">
  <img src="images/logo.png" width="600" />
  <br />
  <a href="https://github.com/dadav/helm-schema/releases"><img src="https://img.shields.io/github/release/dadav/helm-schema.svg" alt="Latest Release"></a>
  <a href="https://pkg.go.dev/github.com/dadav/helm-schema?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="Go Docs"></a>
  <a href="https://github.com/dadav/helm-schema/actions"><img src="https://img.shields.io/github/actions/workflow/status/dadav/helm-schema/build.yml" alt="Build Status"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT LICENSE"></a>
  <a href="https://github.com/pre-commit/pre-commit"><img src="https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit" alt="pre-commit" style="max-width:100%;"></a>
  <a href="https://goreportcard.com/badge/github.com/dadav/helm-schema"><img src="https://goreportcard.com/badge/github.com/dadav/helm-schema" alt="Go Report"></a>
</p>


This tool tries to help you to easily create some nice jsonschema for your helm chart.

By default it will traverse the current directory and look for `Chart.yaml` files.
For every file, helm-schema will try to find one of the given value filenames.
The first files found will be read and a jsonschema will be created.
For every dependency defined in the Chart.yaml file, a reference to the dependencies jsonschema
will be created.

The tool uses jsonschema Draft 7, because the library helm uses only supports that version.

## Installation

Via go install:

```bash
go install github.com/dadav/helm-schema/cmd/helm-schema@latest
```

From aur:

```bash
paru -S helm-schema
```

## Usage

### Pre-commit hook

If you want to automatically generate a new `values.schema.json` if you change the `values.yaml`
file, you can do the following:

Copy the [.pre-commit-config.yaml](./.pre-commit-config.yaml) to your helm chart repository.

Then run this commands:

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
  -c, --chart-search-root string      directory to search recursively within for charts (default ".")
  -x, --dont-strip-helm-docs-prefix   Disable the removal of the helm-docs prefix (--)
  -d, --dry-run                       don't actually create files just print to stdout passed
  -h, --help                          help for helm-schema
  -s, --keep-full-comment             Keep the whole leading comment (default: cut at empty line)
  -l, --log-level string              Level of logs that should printed, one of (panic, fatal, error, warning, info, debug, trace) (default "info")
  -n, --no-dependencies               don't analyze dependencies
  -a, --omit-additional-properties    do not set "additionalProperties": false by default
  -o, --output-file string            jsonschema file path relative to each chart directory to which jsonschema will be written (default "values.schema.json")
  -f, --value-files strings           filenames to check for chart values (default [values.yaml])
  -v, --version                       version for helm-schema
```

## Annotations

The jsonschema must be between two entries of

> `# @schema`

It must be written in front of the key you want to annotate.

If you don't use the `properties` option on hashes or don't use `items` on arrays,
it will be parsed from the values and their annotations instead.

Valid options are:

**type**

Defines the [jsonschema-type](https://json-schema.org/understanding-json-schema/reference/type.html) of the object.

Possible values are:
> object, array, string, number, integer, null

**title**

Defines the [title field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=title) of the object.

Defaults to the key itself.

**description**

Defines the [description field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=description) of the object.

Defaults to the comment which has no `# @schema` prefix.

**default**

Sets the default value and will be displayed first on the users IDE.

**properties**

Contains a map with keys as property names and values as schema.

**pattern**

Regex pattern to test the value.

**format**

The [format keyword](https://json-schema.org/understanding-json-schema/reference/string.html#format) allows for basic semantic identification of certain kinds of string values.


**required**

Adds the key to the required items. 

Takes a boolean value.

**deprecated**

Marks the option as deprecated.

Takes a boolean value.

**items**

Contains the schema that describes the possible array items.

**enum**

Multiple allowed values.

**const**

Single allowed value.

**examples**

Some examples you can provide for the enduser. 

Takes an array.

**minimum**

Minimum value.

Can't be used with `exclusiveMinimum`.

Must be smaller than `maximum` or `exclusiveMaximum` (if used).

**exclusiveMinimum**

Exclusive minimum.

Can't be used with `minimum`.

Must be smaller than `maximum` or `exclusiveMaximum` (if used).

**maximum**

Maximum value.

Can't be used with `exclusiveMaximum`.

Must be bigger than `minimum` or `exclusiveMinimum` (if used).

**exclusiveMaximum**

Exclusive maximum value.

Can't be used with `maximum`.

Must be bigger than `minimum` or `exclusiveMinimum` (if used).

**multipleOf**

Value, the yaml-value must be a multiple of.

For example: If you set this to 10, allowed values would be 0, 10, 20, 30....

**additionalProperties**

Allow additional keys in maps. Useful if you want to use for example `additionalAnnotations`,
which will be filled with keys that the jsonschema can't know.

Defaults to `false` if the map is not an empty map.

Takes a schema or boolean value.

**patternProperties**

Contains a map which maps schemas to pattern. If properties match the patterns, the given
schema is applied.

**anyOf**

Accepts an array of schemas. None or one must apply.

**oneOf**

Accepts an array of schemas. One or more must apply.

**allOf**

Accepts an array of schemas. All must apply.

**if/then/else**

`If` the given schema applies, `then` also apply the given schema or `else` the other schema.

## Example

This values.yaml:

```yaml
---
# vim: set ft=yaml:
# yaml-language-server: $schema=values.schema.json

# This text will be used as description.
# @schema
# type: integer 
# minimum: 0
# @schema
foo: 1

# If you want the value to take alternative values.
# In this case null or some string starting with foo.
# @schema
# anyOf:
#   - type: "null"
#   - pattern: ^foo
# @schema
bar:

# If you don't use `type`, the current value type will be used.
# @schema
# title: Some title
# description: Some description
# @schema
baz: foo

# By default every property is a required property,
# you can disable this with `required=false`
# @schema
# required: false
# @schema
bax: foo

# You can also use conditional settings with if/then/else
# @schema
# anyOf:
#   - type: "null"
#   - type: string
# if:
#   type: "null"
# then:
#   description: It's a null value
# else:
#   description: It's a string
# @schema
bay:

# If you want to specify a schema for possible array values without using a default value,
# do it like this:
# @schema
# type: array
# items:
#   type: object
#   properties:
#     foo:
#       type: integer
#     bar:
#       type: string
# @schema
baz: []
```

## Dependencies

Per default, `helm-schema` will try to also create the schemas for the dependencies in the charts
directory. These schemas will be added as properties in the main schema, but the
`requiredProperties` field will be nullified. Otherwise you would have to always overwrite all the
required fields.

If you don't want to generate jsonschemas for dependencies, you can use the `-n` option.

## Limitations

You can't change the jsonschema for dependencies by using `@schema` annotations on dependency
config values. For example: 

```yaml
# foo is a dependency chart
foo:
  # You can't change the schema here, this has no effect.
  # @schema
  # type: number
  # @schema
  bar: 1
```

## License

[MIT](https://github.com/dadav/helm-schema/blob/main/LICENSE)
