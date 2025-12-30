#!/bin/bash
# Script to sign Helm plugin tarballs for Helm v4 verification
# This creates .prov (provenance) files using GPG signing

set -euo pipefail

PLUGIN_NAME="helm-schema"
VERSION="${1:-}"
TARBALL="${2:-}"
GPG_KEY="${GPG_SIGNING_KEY:-}"
KEYRING="${GPG_KEYRING:-$HOME/.gnupg/pubring.gpg}"

usage() {
    cat <<EOF
Usage: $0 <version> <tarball> [gpg-key]

Signs a Helm plugin tarball with GPG to create a provenance file (.prov)
for Helm v4 plugin verification.

Arguments:
    version     Plugin version (e.g., 1.0.0)
    tarball     Path to the plugin tarball to sign
    gpg-key     GPG key name or email (optional, uses GPG_SIGNING_KEY env var)

Environment Variables:
    GPG_SIGNING_KEY     GPG key to use for signing
    GPG_KEYRING         Path to GPG keyring (default: ~/.gnupg/pubring.gpg)
    GPG_PASSPHRASE      GPG key passphrase (if needed)

Example:
    $0 1.0.0 dist/helm-schema_1.0.0_Linux_x86_64.tar.gz "John Doe <john@example.com>"

EOF
    exit 1
}

if [ -z "$VERSION" ] || [ -z "$TARBALL" ]; then
    usage
fi

if [ ! -f "$TARBALL" ]; then
    echo "Error: Tarball not found: $TARBALL"
    exit 1
fi

# If GPG key not provided as argument, try environment variable
if [ $# -ge 3 ]; then
    GPG_KEY="$3"
fi

if [ -z "$GPG_KEY" ]; then
    echo "Error: GPG signing key not specified"
    echo "Provide it as third argument or set GPG_SIGNING_KEY environment variable"
    exit 1
fi

echo "Signing plugin tarball with GPG..."
echo "  Tarball: $TARBALL"
echo "  Version: $VERSION"
echo "  GPG Key: $GPG_KEY"
echo "  Keyring: $KEYRING"

# Export keys to legacy format if needed (for GnuPG v2)
if ! [ -f "$KEYRING" ]; then
    echo "Exporting GPG keys to legacy format..."
    mkdir -p "$(dirname "$KEYRING")"
    gpg --export > "$KEYRING" 2>/dev/null || true
fi

# Create a temporary directory for signing
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Copy tarball to temp directory
cp "$TARBALL" "$TEMP_DIR/"
TARBALL_NAME=$(basename "$TARBALL")

cd "$TEMP_DIR"

# Create the provenance file
# The provenance file contains:
# 1. The plugin metadata (from plugin.yaml)
# 2. SHA256 hash of the tarball
# 3. GPG signature of the above
echo "Creating provenance file..."

# Extract plugin.yaml from tarball to include in provenance
tar -xzf "$TARBALL_NAME" plugin.yaml 2>/dev/null || tar -xzf "$TARBALL_NAME" */plugin.yaml 2>/dev/null || true

# Calculate SHA256 hash
HASH=$(sha256sum "$TARBALL_NAME" | awk '{print $1}')

# Create provenance content
cat > "${TARBALL_NAME}.prov.tmp" <<EOF
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

name: $PLUGIN_NAME
version: $VERSION
description: Generate jsonschemas for your helm charts
home: https://github.com/dadav/helm-schema

files:
  $TARBALL_NAME: sha256:$HASH
EOF

# If plugin.yaml was extracted, append it
if [ -f plugin.yaml ]; then
    echo "" >> "${TARBALL_NAME}.prov.tmp"
    echo "plugin.yaml: |" >> "${TARBALL_NAME}.prov.tmp"
    sed 's/^/  /' plugin.yaml >> "${TARBALL_NAME}.prov.tmp"
fi

# Sign the provenance file
if [ -n "${GPG_PASSPHRASE:-}" ]; then
    # Use passphrase from environment if available
    echo "$GPG_PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 \
        --armor \
        --detach-sign \
        --local-user "$GPG_KEY" \
        --output "${TARBALL_NAME}.prov.sig" \
        "${TARBALL_NAME}.prov.tmp"
else
    # Interactive passphrase prompt
    gpg --armor \
        --detach-sign \
        --local-user "$GPG_KEY" \
        --output "${TARBALL_NAME}.prov.sig" \
        "${TARBALL_NAME}.prov.tmp"
fi

# Combine into final .prov file (clearsigned format)
cat "${TARBALL_NAME}.prov.tmp" > "${TARBALL_NAME}.prov"
echo "" >> "${TARBALL_NAME}.prov"
cat "${TARBALL_NAME}.prov.sig" >> "${TARBALL_NAME}.prov"

# Copy back to original location
cp "${TARBALL_NAME}.prov" "$(dirname "$TARBALL")/"

echo "✓ Successfully created provenance file: ${TARBALL}.prov"
echo ""
echo "To verify the signature:"
echo "  helm plugin verify $(basename "$TARBALL")"
echo ""
echo "To install with verification:"
echo "  helm plugin install $(basename "$TARBALL") --verify"
