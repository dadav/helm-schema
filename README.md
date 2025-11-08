# helm-schema

<p align="center">
  <img src="images/logo.png" width="400" />
  <br />
  <a href="https://github.com/dadav/helm-schema/releases"><img src="https://img.shields.io/github/release/dadav/helm-schema.svg" alt="Latest Release"></a>
  <a href="https://pkg.go.dev/github.com/dadav/helm-schema?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="Go Docs"></a>
  <a href="https://github.com/dadav/helm-schema/actions"><img src="https://img.shields.io/github/actions/workflow/status/dadav/helm-schema/build_and_test.yml" alt="Build Status"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT LICENSE"></a>
  <a href="https://github.com/pre-commit/pre-commit"><img src="https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit" alt="pre-commit" style="max-width:100%;"></a>
  <a href="https://goreportcard.com/badge/github.com/dadav/helm-schema"><img src="https://goreportcard.com/badge/github.com/dadav/helm-schema" alt="Go Report"></a>
</p>

<p align="center">This tool tries to help you to easily create some nice <a href="https://json-schema.org/" target="_blank"><strong>JSON schema</strong></a> for your helm chart.</p>

By default it will traverse the current directory and look for `Chart.yaml` files.
For every file, helm-schema will try to find one of the given value filenames.
The first files found will be read and a jsonschema will be created.
For every dependency defined in the `Chart.yaml` file, a reference to the dependencies JSON schema
will be created.

> [!NOTE]
> The tool uses `jsonschema` Draft 7, because the library helm uses only supports that version.

## Installation

Via `go` install:

```sh
go install github.com/dadav/helm-schema/cmd/helm-schema@latest
```

From `aur`:

```sh
paru -S helm-schema
```

Via `podman/docker`:

```sh
podman run --rm -v $PWD:/home/helm-schema ghcr.io/dadav/helm-schema:latest
```

As `helm plugin`:

```sh
helm plugin install https://github.com/dadav/helm-schema
```

## Usage

### Pre-commit hook

If you want to automatically generate a new `values.schema.json` if you change the `values.yaml`
file, you can do the following:

1. Install [`pre-commit`](https://pre-commit.com/#install)
2. Copy the [`.pre-commit-config.yaml`](./.pre-commit-config.yaml) to your helm chart repository.
3. Then run these commands:

```sh
pre-commit install
pre-commit install-hooks
```

### Running the binary directly

You can also just run the binary yourself:

```sh
helm-schema
```

### Options

The binary has the following options:

```sh
Flags:
  -r, --add-schema-reference                   "add reference to schema in values.yaml if not found"
  -w, --allow-circular-dependencies            "allow circular dependencies between charts (will log a warning instead of failing)"
  -a, --append-newline                         "append newline to generated jsonschema at the end of the file"
  -c, --chart-search-root string               "directory to search recursively within for charts (default ".")"
  -i, --dependencies-filter strings            "only generate schema for specified dependencies (comma-separated list of dependency names)"
  -g, --dont-add-global                        "dont auto add global property"
  -x, --dont-strip-helm-docs-prefix            "disable the removal of the helm-docs prefix (--)"
  -d, --dry-run                                "don't actually create files just print to stdout passed"
  -p, --helm-docs-compatibility-mode           "parse and use helm-docs comments"
  -h, --help                                   "help for helm-schema"
  -s, --keep-full-comment                      "keep the whole leading comment (default: cut at empty line)"
  -l, --log-level string                       "level of logs that should printed, one of (panic, fatal, error, warning, info, debug, trace) (default "info")"
  -n, --no-dependencies                        "don't analyze dependencies"
  -o, --output-file string                     "jsonschema file path relative to each chart directory to which jsonschema will be written (default 'values.schema.json')"
  -m, --skip-dependencies-schema-validation    "skip schema validation for dependencies by setting additionalProperties to true and removing from required"
  -f, --value-files strings                    "filenames to check for chart values (default [values.yaml])"
  -k, --skip-auto-generation strings           "skip the auto generation for these fields (default [])"
  -u, --uncomment                              "consider yaml which is commented out"
  -v, --version                                "version for helm-schema"
```

## Annotations

The `jsonschema` must be between two entries of `# @schema` :

```yaml
# @schema
# my: annotation
# @schema
# you can add comment here as well
foo: bar
```

> [!WARNING]
> It must be written just above the key you want to annotate.

> [!NOTE]
> If you don't use the `properties` option on hashes/objects or don't use `items` on arrays, it will be parsed from the values and their annotations instead.

### Available annotations

<!-- prettier-ignore -->
| Key| Description | Values |
|-|-|-|
| [`type`](#type) | Defines the [jsonschema-type](https://json-schema.org/understanding-json-schema/reference/type.html) of the object. Multiple values are supported (e.g. `[string, integer]`) as a shortcut to `anyOf` | `object`, `array`, `string`, `number`, `integer`, `boolean` or `null` |
| [`title`](#title) | Defines the [title field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=title) of the object | Defaults to the key itself |
| [`description`](#description) | Defines the [description field](https://json-schema.org/understanding-json-schema/reference/generic.html?highlight=description) of the object. | Defaults to the comments just above or below the `@schema` annotations block |
| [`default`](#default) | Sets the default value and will be displayed first on the users IDE| Takes a `string` |
| [`properties`](#properties) | Contains a map with keys as property names and values as schema | Takes an `object` |
| [`pattern`](#pattern) | Regex pattern to test the value | Takes an `string` |
| [`format`](#format) | The [format keyword](https://json-schema.org/understanding-json-schema/reference/string.html#format) allows for basic semantic identification of certain kinds of string values | Takes a [keyword](https://json-schema.org/understanding-json-schema/reference/string.html#format) |
| [`required`](#required) | Adds the key to the required items | `true` or `false` or `array` |
| [`deprecated`](#deprecated) | Marks the option as deprecated | `true` or `false` |
| [`items`](#items) | Contains the schema that describes the possible array items | Takes an `object` |
| [`enum`](#enum) | Multiple allowed values. Accepts an array of `string` | Takes an `array` |
| [`const`](#const) | Single allowed value | Takes a `string`|
| [`examples`](#examples) | Some examples you can provide for the end user | Takes an `array` |
| [`minimum`](#minimum) | Minimum value. Can't be used with `exclusiveMinimum` | Takes an `integer`. Must be smaller than `maximum` or `exclusiveMaximum` (if used) |
| [`exclusiveMinimum`](#exclusiveminimum) | Exclusive minimum. Can't be used with `minimum` | Takes an `integer`. Must be smaller than `maximum` or `exclusiveMaximum` (if used) |
| [`maximum`](#maximum) | Maximum value. Can't be used with `exclusiveMaximum` | Takes an `integer`. Must be bigger than `minimum` or `exclusiveMinimum` (if used) |
| [`exclusiveMaximum`](#exclusivemaximum) | Exclusive maximum value. Can't be used with `maximum` | Takes an `integer`. Must be bigger than `minimum` or `exclusiveMinimum` (if used) |
| [`multipleOf`](#multipleof) | The yaml-value must be a multiple of. For example: If you set this to 10, allowed values would be 0, 10, 20, 30... | Takes an `integer` |
| [`additionalProperties`](#additionalproperties) | Allow additional keys in maps. Useful if you want to use for example `additionalAnnotations`, which will be filled with keys that the `jsonschema` can't know| Defaults to `false` if the map is not an empty map. Takes a schema or boolean value |
| [`patternProperties`](#patternproperties) | Contains a map which maps schemas to pattern. If properties match the patterns, the given schema is applied| Takes an `object` |
| [`anyOf`](#anyof) | Accepts an array of schemas. None or one must apply | Takes an `array` |
| [`oneOf`](#oneof) | Accepts an array of schemas. One or more must apply | Takes an `array` |
| [`allOf`](#allof) | Accepts an array of schemas. All must apply| Takes an `array` |
| [`not`](#not) | A schema that must not be matched. | Takes an `object` |
| [`if/then/else`](#ifthenelse) | `if` the given schema applies, `then` also apply the given schema or `else` the other schema| Takes an `object` |
| [`$ref`](#ref) | Accepts an URI to a valid `jsonschema`. Extend the schema for the current key | Takes an URI (or relative file) |
| [`minLength`](#minlength) | Minimum string length. | Takes an `integer`. Must be smaller or equal than `maxLength` (if used) |
| [`maxLength`](#maxlength) | Maximum string length. | Takes an `integer`. Must be greater or equal than `minLength` (if used) |
| [`minItems`](#minItems) | Minimum length of an array. | Takes an `integer`. Must be smaller or equal than `maxItems` (if used) |
| [`maxItems`](#maxItems) | Maximum length of an array. | Takes an `integer`. Must be greater or equal than `minItems` (if used) |

## Validation & completion

To take advantage of the generated `values.schema.json`, you can use it within your IDE through a plugin supporting the `yaml-language-server` annotation (e.g. [VSCode - YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml))

You'll have to place this line at the top of your `values.yaml` (`$schema=<path-or-url-to-your-schema>`) :

```yaml
# vim: set ft=yaml:
# yaml-language-server: $schema=values.schema.json

# @schema
# required: true
# @schema
# -- This is an example description
foo: bar
```

You can use the `-r` flag to make sure this line exists.

> [!NOTE]
> You can also point to an online available schema, if you upload a version of yours and want other to be able to implement it.
>
> `yaml-language-server: $schema=https://example.org/my-json-schema.json`
>
> e.g. from github `https://raw.githubusercontent.com/<user>/<repo>/main/values.schema.json`

### helm-docs

If you're using [`helm-docs`](https://github.com/norwoodj/helm-docs), then you can combine both annotations and use both pre-commit hooks to automatically generate your documentation (e.g. `README.md`) alongside your `values.schema.json`.

If not provided, `title` will be the key and the `description` will be parsed from the `helm-docs` formatted comment.

```yaml
# @schema
# type: array
# @schema
# -- helm-docs description here
foo: []
```

If you use `-p`/`--helm-docs-compatibility-mode` flags, the `@default`, `(type)` annotations and helm-docs descriptions
are used if detected.

> [!NOTE]
> Make sure to place the `@schema` annotations **before** the actual key description to avoid having it in your `helm-docs` generated table

## Dependencies

Per default, `helm-schema` will try to also create the schemas for the dependencies in their respective chart directory. These schemas will be merged as properties in the main schema, but the `requiredProperties` field will be nullified, otherwise you would have to always overwrite all the required fields.

If you don't want to generate `jsonschema` for chart dependencies, you can use the `-n, --no-dependencies` option to only generate the `values.schema.json` for your parent chart(s)

### Skip Dependency Schema Validation

By default, when dependency schemas are merged into the parent chart schema, they inherit strict validation rules. This means that if you add unknown keys at the top level of a dependency's values (e.g., `subchart.unknownKey`), validation may not fail as expected.

If you want to allow additional properties in dependency schemas and ensure they are not required, you can use the `-m, --skip-dependencies-schema-validation` flag. This will:

1. Set `additionalProperties: true` for all dependency schemas in the parent chart
2. Remove dependency names from the parent chart's required properties list

Example usage:

```sh
helm-schema -m
```

This is useful when you have umbrella charts with multiple dependencies and want to allow flexibility in overriding dependency values without strict schema validation.

### Handling Circular Dependencies

In some scenarios, you may have charts that reference each other to share values, creating circular dependencies. For example:
- A cert-manager chart depends on a grafana chart to get the instance name for creating dashboards
- The grafana chart depends on the cert-manager chart to get the ACME issuer value

By default, `helm-schema` will detect this as a circular dependency and log a warning. The tool will still continue processing, but the results may not be sorted in dependency order.

If you want to explicitly allow circular dependencies and acknowledge this behavior, you can use the `-w, --allow-circular-dependencies` flag:

```sh
helm-schema -w
```

When this flag is enabled, circular dependencies are treated as warnings rather than errors, and the charts are processed in their original discovery order instead of topologically sorted order.

**Note:** This is primarily useful when charts have cross-dependencies purely for value sharing, not for actual build order dependencies.

## Limitations

You can't change the `jsonschema` for dependencies by using `@schema` annotations on dependency config values. For example:

```yaml
# foo is a dependency chart
foo:
  # You can't change the schema here, this has no effect.
  # @schema
  # type: number
  # @schema
  bar: 1
```

## Examples

Some annotation examples you may want to use, to help you get started!

> [!NOTE]
> See how the schema behaves with live examples : [values.yaml](./examples/values.yaml)

Below a snippet to test it out, with the current options `helm-schema` will not analyze dependencies (`-n`) and will omit the `additionalProperties` (`-k`, when not explicitly defined) in the generated schema. It will start looking for `Chart.yaml` and `values.yaml` files in `examples/` (`-c`)

```sh
cd examples
helm-schema -n -k additionalProperties

# or

helm-schema -c examples -n -k additionalProperties
```

If you'd like to use `helm-schema` on your chart dependencies as well, you have to build and unpack them before. You'll avoid the "missing dependency" error message.

```sh
# go where your Chart.lock/yaml is located
cd <chart-name>

# build dependencies and untar them
helm dep build
ls charts/*.tgz |xargs -n1 tar -C charts/ -xzf
```

#### `type`

If `type` isn't specified, current value type will be used.

```yaml
# Will be parsed as 'string'
# @schema
# title: Some title
# description: Some description
# @schema
name: foo

# Will be parsed as 'boolean'
# @schema
# type: boolean
# @schema
enabled: true

# You can define multiple types as an array.
# @schema
# type: [string, integer]
# minimum: 0
# @schema
cpu: 1
```

#### `title`

By default, the `title` will be parsed from the key name. If the key is `foo`, then `title: foo`.

```yaml
# Define a custom title for the key
# @schema
# title: My custom title for 'foo'
# @schema
bar: foo
```

#### `description`

You can provide the `description` through its property or let it be parsed from your comments. If `description` is provided, the comments will not be parsed as description.

If you're implementing it alongside `helm-docs`, read [this](#helm-docs) to do it correctly.

```yaml
# This text will be used as description.
# @schema
# type: integer
# minimum: 1
# @schema
replica: 1

# @schema
# type: integer
# minimum: 1
# @schema
# This text will be used as description.
replica: 1

# @schema
# type: integer
# minimum: 1
# description: This text will be used as description.
# @schema
# And not this one
replica: 1
```

#### `default`

Help users when using their IDE to quickly retrieve the `default` value, for example through <kbd>CTRL+SPACE</kbd>.

```yaml
# @schema
# default: standalone
# enum: [standalone,cluster]
# @schema
architecture: ""

# @schema
# type: boolean
# default: true
# @schema
enabled: true
```

#### `properties`

Allows user to define valid keys without defining them yet. Give the user an insight of the possible properties, their types and description.

By default, `title` for the keys defined under `properties` will inherit from the main key (e.g. here `title: env`). You need to provide `title` explicitly if you want to change it.

```yaml
# @schema
# properties:
#   CONFIG_PATH:
#     title: CONFIG_PATH
#     type: string
#     description: The local path to the service configuration file
#   ADMIN_EMAIL:
#     title: ADMIN_EMAIL
#     type: string
#     format: idn-email
#   API_URL:
#     type: string
#     format: idn-hostname
#     description: Title will be 'env' as we do not specify it here
# @schema
# -- Environment variables. If you want to provide auto-completion to the user
env: {}
```

#### `pattern`

Pattern that'll be used to test the value.

```yaml
# @schema
# pattern: ^api-key
# @schema
# The value have to start with the 'api-key-' prefix
apiKey: "api-key-xxxxx"
```

#### `format`

Known formats that the value must match. Formats available at [JSON Schema - Formats](https://json-schema.org/understanding-json-schema/reference/string.html#format).

```yaml
# @schema
# format: idn-email
# @schema
# Requires a valid email format
email: foo@example.org
```

#### `required`

By default every property is a required property, you can disable this with `required: false` for a single key. You can also invert this behaviour with the option `helm-schema -k required`, now every property is an optional one.

```yaml
# @schema
# required: false
# @schema
altName: foo
```

It's also possible to define an array of required properties on the parent.

```yaml
# @schema
# required: [foo]
# @schema
altName:
  foo: bar
```

#### `deprecated`

Let the user know if the key is deprecated, hence should be avoided.

```yaml
# @schema
# deprecated: true
# @schema
secret: foo
```

#### `items`

If you want to specify a schema for possible array values without using a default value. E.g. to define the structure of the hosts definition in an k8s ingress resource.

```yaml
# @schema
# type: array
# items:
#   type: object
#   properties:
#     host:
#       type: object
#       properties:
#         url:
#           type: string
#           format: idn-hostname
# @schema
# Will give auto-completion for the below structure
# hosts:
#  - name:
#      url: my.example.org
hosts: []
```

#### `enum`

Allows user to define available values for a given key. Validation will fail and error shown if you try to put another value.

```yaml
# @schema
# enum:
# - application
# - controller
# - api
# @schema
# Only those three values are accepted
type: application

# @schema
# type: array
# items:
#   enum: [api,frontend,backend,microservice,teamA,teamB,us-west-1,us-west-2]
# @schema
# For each array index, only one of those values are accepted
tags:
  - "api"
  - "teamA"
  - "us-west-2"
```

#### `const`

Defines a constant value which shouldn't be changed.

```yaml
# @schema
# const: maintainer@example.org
# @schema
maintainer: maintainer@example.org
```

#### `examples`

Provides example values to the user when hovering the key in IDE, or by auto-completion mechanism.

```yaml
# @schema
# format: ipv4
# examples: [192.168.0.1]
# @schema
clusterIP: ""

# @schema
# properties:
#   CONFIG_PATH:
#     type: string
#     description: The local path to the service configuration file
#     examples: [/path/to/config]
#   ADMIN_EMAIL:
#     type: string
#     format: idn-email
#     examples: [admin@example.org]
#   API_URL:
#     type: string
#     format: idn-hostname
#     examples: [https://api.example.org]
# @schema
# -- Provide auto-completion and examples to the user
env: {}
```

#### `minimum`

The value have to be above or equal the given `integer`.

```yaml
# @schema
# minimum: 1
# @schema
replica: ""
```

#### `exclusiveMinimum`

The value have to be strictly above the given `integer`.

```yaml
# @schema
# exclusiveMinimum: 0
# @schema
replica: ""
```

#### `maximum`

The value have to be below or equal the given `integer`.

```yaml
# @schema
# maximum: 10
# @schema
replica: ""
```

#### `exclusiveMaximum`

The value have to be strictly below the given `integer`.

```yaml
# @schema
# exclusiveMaximum: 5
# @schema
cpu: ""
```

#### `multipleOf`

The value have to be a multiple of the given `integer`.

```yaml
# @schema
# multipleOf: 1024
# @schema
storageCapacity: 2048
```

#### `additionalProperties`

By default, `additionalProperties` is set to `false` unless you use the `-k additionalProperties` option. Useful when you don't know what nested keys you'll have.

```yaml
# @schema
# additionalProperties: true
# @schema
# You'll be able to add as many keys below `env:` as you want without invalidating the schema
env:
  LONG: foo
  LIST: bar
  OF: baz
  VARIABLES: bat

# @schema
# additionalProperties: true
# properties:
#   REQUIRED_VAR:
#     type: string
# @schema
env:
  REQUIRED_VAR: foo
  OPTIONAL_VAR: bar
```

#### `patternProperties`

Mapping schemas to key name patterns. If properties match the patterns, the given schema is applied.

Useful when you work with a long list of keys and want to define a common schema for a group of them, for example.

E.g. `patternProperties."^API_.*"` key defines the pattern whose schema will be applied on any user provided key that match that pattern.

```yaml
# @schema
# type: object
# patternProperties:
#   "^API_.*":
#     type: string
#     pattern: ^api-key
#   "^EMAIL_.*":
#     type: string
#     format: idn-email
# @schema
env:
  API_PROVIDER_ONE: api-key-xxxxx
  API_PROVIDER_TWO: api-key-xxxxx
  EMAIL_ADMIN: admin@example.org
  EMAIL_DEFAULT_USER: user@example.org
```

#### `anyOf`

Allows user to define multiple schema fo a single key. Key can be `anyOf` the given schemas or none of them.

```yaml
# Accepts multiple types
# @schema
# anyOf:
#   - type: string
#   - type: integer
# minimum: 0
# @schema
foot: 1

# The above can be simplified with `type:`
# @schema
# type: [string, integer]
# minimum: 0
# @schema
fool: 1

# A pattern is also possible.
# In this case null or some string starting with foo.
# @schema
# anyOf:
#   - type: "null"
#   - pattern: ^foo
# @schema
bar:
```

#### `oneOf`

Allows user to define multiple schema fo a single key. Key must match `oneOf` the given schemas.

```yaml
# @schema
# oneOf:
#   - type: integer
#   - pattern: Gib$
#   - pattern: gib$
# @schema
storage: 30Gib
```

#### `allOf`

Allows user to define multiple schema for a single key. Key must match `oneOf` the given schemas.

```yaml
# @schema
# allOf:
#   - type: string
#     pattern: Gib$
#   - enum: [5Gib,10Gib,15Gib]
# @schema
storage: 10Gib
```

#### `not`

Allows to define a schema that must not be matched.

```yaml
# @schema
# not:
#   type: string
# @schema
foo: bar
```

#### `if/then/else`

Conditional schema settings with `if`/`then`/`else`

```yaml
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
unknown: foo
```

#### `minLength`

The value must be an integer greater or equal to zero and defines the minimum length of a string value.

```yaml
# @schema
# minLength: 1
# @schema
namespace: foo
```

#### `maxLength`

The value must be an integer greater than zero and defines the maximum length of a string value.

```yaml
# @schema
# maxLength: 3
# @schema
namespace: foo
```

#### `minItems`

The value must be an integer greater than zero and defines the minimum length of an array value.

```yaml
# @schema
# minItems: 1
# @schema
namespace:
  - foo
```

#### `maxItems`

The value must be an integer greater than zero and defines the maximum length of an array value.

```yaml
# @schema
# maxItems: 2
# @schema
namespace:
  - foo
  - bar
```

#### `uniqueItems`

A schema can ensure that each of the items in an array is unique. Simply set the uniqueItems keyword to true.

```yaml
# @schema
# uniqueItems: true
# @schema
namespace:
  - foo
  - bar
```

#### `$ref`

The value must be an URI or relative file.

Relative files are imported on creation time. If you update the referenced file, you need
to run helm-schema again.

**foo.json:**

```json
{
  "foo": {
    "type": "string",
    "minLength": 10
  }
}
```

```yaml
# @schema
# $ref: foo.json#/foo
# @schema
namespace: foo
```

is the same as

```yaml
# @schema
# type: string
# minLength: 10
# @schema
namespace: foo
```

## License

[MIT](https://github.com/dadav/helm-schema/blob/main/LICENSE)
