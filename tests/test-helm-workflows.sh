#!/usr/bin/env bash
set -euo pipefail

test_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
schema_binary="$test_dir/helm-schema"
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

export HELM_REPOSITORY_CONFIG="$work_dir/repositories.yaml"
export HELM_REPOSITORY_CACHE="$work_dir/repository"
printf 'repositories: []\n' >"$HELM_REPOSITORY_CONFIG"
mkdir -p "$HELM_REPOSITORY_CACHE"

command -v helm >/dev/null
command -v jq >/dev/null

cp -R "$test_dir/import-values" "$work_dir/import-values"

for parent in parent parent-complex; do
	chart_dir="$work_dir/import-values/$parent"
	if [[ "$parent" == parent-complex ]]; then
		printf '    alias: backend\n' >>"$chart_dir/Chart.yaml"
	fi
	mkdir -p "$chart_dir/templates"
	cat >"$chart_dir/templates/configmap.yaml" <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}
data:
  values: {{ .Values | toJson | quote }}
EOF

	# Local file dependencies are packaged before generation, as in chart CI.
	helm dependency build "$chart_dir" --skip-refresh
	"$schema_binary" -c "$chart_dir"
	"$schema_binary" -c "$chart_dir" --check
	"$schema_binary" -c "$chart_dir" -K
	"$schema_binary" -c "$chart_dir" -K --check
	helm lint "$chart_dir"
	helm template smoke "$chart_dir" >/dev/null

	if [[ "$parent" == parent ]]; then
		invalid_value=child.exports.defaults.dbPort=invalid
	else
		invalid_value=backend.data.database.maxConnections=invalid
	fi
	if helm template smoke "$chart_dir" --set "$invalid_value" >"$work_dir/invalid.log" 2>&1; then
		echo "Expected schema validation to reject $invalid_value" >&2
		exit 1
	fi
	grep -q 'values don.t meet the specifications' "$work_dir/invalid.log"
	echo "Validated $parent with packaged dependencies and an invalid override"
done

# README quickstart: generation, validation, editor reference, and freshness.
chart_dir="$work_dir/schema-demo"
mkdir -p "$chart_dir/templates"
cat >"$chart_dir/Chart.yaml" <<'EOF'
apiVersion: v2
name: schema-demo
version: 0.1.0
EOF
cat >"$chart_dir/values.yaml" <<'EOF'
# @schema
# type: integer
# minimum: 1
# required: true
# @schema
# Number of application replicas.
replicaCount: 1
EOF
cat >"$chart_dir/templates/configmap.yaml" <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}
data:
  replicaCount: {{ .Values.replicaCount | quote }}
EOF

"$schema_binary" -c "$chart_dir" --add-schema-reference
"$schema_binary" -c "$chart_dir" --check
helm lint "$chart_dir"
helm template demo "$chart_dir" >/dev/null
if helm template demo "$chart_dir" --set replicaCount=0 >"$work_dir/invalid.log" 2>&1; then
	echo "Expected the README quickstart to reject replicaCount=0" >&2
	exit 1
fi
grep -q 'values don.t meet the specifications' "$work_dir/invalid.log"

cat >"$chart_dir/values.prod.yaml" <<'EOF'
replicaCount: 3
EOF
"$schema_binary" -c "$chart_dir" -f values.yaml,values.prod.yaml
"$schema_binary" -c "$chart_dir" -f values.yaml,values.prod.yaml --check
jq -e '.properties.replicaCount.default == 3' "$chart_dir/values.schema.json" >/dev/null
helm template demo "$chart_dir" -f "$chart_dir/values.prod.yaml" >/dev/null

: >"$chart_dir/values.empty.yaml"
"$schema_binary" -c "$chart_dir" -f values.yaml,values.empty.yaml
"$schema_binary" -c "$chart_dir" -f values.yaml,values.empty.yaml --check
helm template demo "$chart_dir" -f "$chart_dir/values.empty.yaml" >/dev/null
echo "Validated README quickstart and values-file recipes"
