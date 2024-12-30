#!/bin/bash

rc=0

cp ../examples/values.yaml test_repo_example.yaml
cp ../examples/values.schema.json test_repo_example_expected.schema.json

for test_file in test_*.yaml; do
	expected_file="${test_file%.yaml}_expected.schema.json"
	generated_file="${test_file%.yaml}_generated.schema.json"
	if ! ./helm-schema -f "$test_file" -o "$generated_file"; then
		echo "❌: $test_file"
		rc=1
		continue
	fi
	echo "Testing $test_file"
	if diff -y --suppress-common-lines <(jq --sort-keys . "$generated_file") <(jq --sort-keys . "$expected_file"); then
		echo "✅: $test_file"
	else
		echo "❌: $test_file"
		rc=1
	fi
done

exit "$rc"
