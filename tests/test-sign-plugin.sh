#!/bin/bash
# Test script for sign-plugin.sh
# Creates an isolated GPG environment, generates a test key, and validates signing

set -euo pipefail

TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT

echo "=== Setting up isolated test environment in $TEST_DIR ==="

# Create isolated GPG home
export GNUPGHOME="$TEST_DIR/gnupg"
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

# Configure GPG for non-interactive use
cat > "$GNUPGHOME/gpg.conf" <<EOF
no-tty
batch
pinentry-mode loopback
EOF

cat > "$GNUPGHOME/gpg-agent.conf" <<EOF
allow-loopback-pinentry
EOF

# Restart gpg-agent with new config
gpgconf --kill gpg-agent 2>/dev/null || true

echo "=== Generating test Ed25519 key ==="
# Generate an Ed25519 key (the type that was causing issues)
gpg --batch --yes --passphrase "testpass" --quick-gen-key "Test User <test@example.com>" ed25519 sign 0

# Get the key fingerprint
KEY_FPR=$(gpg --list-keys --with-colons | grep fpr | head -1 | cut -d: -f10)
echo "Generated key: $KEY_FPR"

# Export to legacy format
gpg --export > "$GNUPGHOME/pubring.gpg"

echo "=== Creating mock plugin tarball ==="
PLUGIN_DIR="$TEST_DIR/plugin"
mkdir -p "$PLUGIN_DIR/bin"

cat > "$PLUGIN_DIR/plugin.yaml" <<EOF
---
name: "test-plugin"
version: "1.0.0"
usage: "test plugin"
description: "A test plugin for signing verification"
command: "\$HELM_PLUGIN_DIR/bin/test-plugin"
EOF

cat > "$PLUGIN_DIR/bin/test-plugin" <<'EOF'
#!/bin/bash
echo "Hello from test plugin"
EOF
chmod +x "$PLUGIN_DIR/bin/test-plugin"

# Create tarball
TARBALL="$TEST_DIR/test-plugin_1.0.0_Linux_x86_64.tar.gz"
tar -czf "$TARBALL" -C "$PLUGIN_DIR" .

echo "=== Running sign-plugin.sh ==="
export GPG_KEYRING="$GNUPGHOME/pubring.gpg"
export GPG_PASSPHRASE="testpass"

# Copy the sign-plugin.sh to test dir (assuming it's in current directory or provided)
if [ -f "sign-plugin.sh" ]; then
    cp sign-plugin.sh "$TEST_DIR/"
elif [ -f "$1" ]; then
    cp "$1" "$TEST_DIR/sign-plugin.sh"
else
    echo "Error: sign-plugin.sh not found. Provide path as argument."
    exit 1
fi

chmod +x "$TEST_DIR/sign-plugin.sh"

# Run the signing script
cd "$TEST_DIR"
./sign-plugin.sh "1.0.0" "$TARBALL" "test@example.com"

echo ""
echo "=== Checking generated .prov file ==="
PROV_FILE="${TARBALL}.prov"

if [ ! -f "$PROV_FILE" ]; then
    echo "FAIL: .prov file not created"
    exit 1
fi

echo "Contents of .prov file:"
echo "---"
cat "$PROV_FILE"
echo "---"

echo ""
echo "=== Verifying signature with GPG ==="
if gpg --verify "$PROV_FILE" 2>&1; then
    echo ""
    echo "SUCCESS: GPG verification passed"
else
    echo ""
    echo "FAIL: GPG verification failed"
    exit 1
fi
echo "=== Verifying plugin with Helm ==="
if helm plugin verify "$TARBALL" 2>&1; then
    echo ""
    echo "SUCCESS: Helm plugin verification passed"
else
    echo ""
    echo "FAIL: Helm plugin verification failed"
    exit 1
fi

echo ""
echo "=== Checking signature packet details ==="
sed -n '/-----BEGIN PGP SIGNATURE-----/,/-----END PGP SIGNATURE-----/p' "$PROV_FILE" | gpg --list-packets 2>&1 | grep -E "(algo|digest)" || true

echo ""
echo "=== All tests passed ==="