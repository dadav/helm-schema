#!/bin/bash

rc=0

cd charts

cp ../../examples/values.yaml test_repo_example.yaml
cp ../../examples/values.schema.json test_repo_example_expected.schema.json

for test_file in test_*.yaml; do
	# Skip annotate test files from normal schema generation tests
	case "$test_file" in
		test_annotate_*) continue ;;
	esac

	expected_file="${test_file%.yaml}_expected.schema.json"
	generated_file="${test_file%.yaml}_generated.schema.json"
	if ! ../helm-schema -f "$test_file" -o "$generated_file"; then
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

# Annotate test
echo "Testing annotate mode"
annotate_output=$(../helm-schema --annotate -d -f test_annotate_input.yaml 2>/dev/null)
if diff -y --suppress-common-lines <(echo "$annotate_output") test_annotate_expected.yaml; then
	echo "✅: annotate mode"
else
	echo "❌: annotate mode"
	rc=1
fi

# Import-values tests (in separate directory to avoid interference with single-file tests)
echo "Testing import-values (simple form)"
../helm-schema -c ../import-values >/dev/null 2>&1
if diff -y --suppress-common-lines <(jq --sort-keys . ../import-values/parent/values.schema.json) <(jq --sort-keys . ../import-values/parent/values.schema.expected.json); then
	echo "✅: import-values (simple form)"
else
	echo "❌: import-values (simple form)"
	rc=1
fi

echo "Testing import-values (complex form)"
if diff -y --suppress-common-lines <(jq --sort-keys . ../import-values/parent-complex/values.schema.json) <(jq --sort-keys . ../import-values/parent-complex/values.schema.expected.json); then
	echo "✅: import-values (complex form)"
else
	echo "❌: import-values (complex form)"
	rc=1
fi

rm -f ../import-values/parent/values.schema.json ../import-values/child/values.schema.json
rm -f ../import-values/parent-complex/values.schema.json ../import-values/child-complex/values.schema.json

# Pre-existing schema test
echo "Testing pre-existing dependency schema"
dep_schema_before=$(cat ../preexisting-schema/dep-with-schema/values.schema.json)
../helm-schema -c ../preexisting-schema >/dev/null 2>&1
if diff -y --suppress-common-lines <(jq --sort-keys . ../preexisting-schema/parent/values.schema.json) <(jq --sort-keys . ../preexisting-schema/parent/values.schema.expected.json); then
	echo "✅: pre-existing dependency schema"
else
	echo "❌: pre-existing dependency schema"
	rc=1
fi

dep_schema_after=$(cat ../preexisting-schema/dep-with-schema/values.schema.json)
if [ "$dep_schema_before" = "$dep_schema_after" ]; then
	echo "✅: dependency schema not overwritten"
else
	echo "❌: dependency schema was overwritten"
	rc=1
fi

rm -f ../preexisting-schema/parent/values.schema.json

exit "$rc"
