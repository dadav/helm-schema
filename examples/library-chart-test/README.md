# Library Chart Test

This test validates that library chart dependencies (type: library) have their schema properties merged at the top level of the parent chart's schema, rather than being nested under the dependency name.

## Expected Behavior

For a parent chart with a library dependency named "common":
- Properties from the library chart (environment, region) should appear at the top level of the parent's schema
- No "common" key should be present in the schema
- Parent properties and library properties should coexist at the same level

## Contrast with Application Dependencies

Unlike application type dependencies (type: application), which are nested under their dependency name, library charts should have their properties merged directly into the parent schema.

This behavior aligns with how Helm library charts work in practice, where the values scope is identical to the parent chart.
