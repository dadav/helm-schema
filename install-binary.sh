#!/usr/bin/env sh

# Shamelessly copied from https://github.com/technosophos/helm-template/blob/master/install-binary.sh

PROJECT_NAME="helm-schema"
BINARY_NAME="helm-schema"
PROJECT_GH="dadav/$PROJECT_NAME"
PLUGIN_MANIFEST="plugin.yaml"

# Convert HELM_BIN and HELM_PLUGIN_DIR to unix if cygpath is
# available. This is the case when using MSYS2 or Cygwin
# on Windows where helm returns a Windows path but we
# need a Unix path

if command -v cygpath >/dev/null 2>&1; then
  HELM_BIN="$(cygpath -u "${HELM_BIN}")"
  HELM_PLUGIN_DIR="$(cygpath -u "${HELM_PLUGIN_DIR}")"
fi

[ -z "$HELM_BIN" ] && HELM_BIN=$(command -v helm)

[ -z "$HELM_HOME" ] && HELM_HOME=$(helm env | grep 'HELM_DATA_HOME' | cut -d '=' -f2 | tr -d '"')

mkdir -p "$HELM_HOME"

if [ "$SKIP_BIN_INSTALL" = "1" ]; then
  echo "Skipping binary install"
  exit
fi

# which mode is the common installer script running in.
SCRIPT_MODE="install"
if [ "$1" = "-u" ]; then
  SCRIPT_MODE="update"
fi

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
  armv6*) ARCH="armv6" ;;
  armv7*) ARCH="armv7" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  x86_64 | amd64) ARCH="x86_64" ;;
  *)
    echo "Arch '$(uname -m)' not supported!" >&2
    exit 1
    ;;
  esac
}

# initOS discovers the operating system for this system.
initOS() {
  OS=$(uname -s)

  case "$OS" in
  Windows_NT) OS='Windows' ;;
  # Msys support
  MSYS*) OS='Windows' ;;
  # Minimalist GNU for Windows
  MINGW*) OS='Windows' ;;
  CYGWIN*) OS='Windows' ;;
  Darwin) OS='Darwin' ;;
  Linux) OS='Linux' ;;
  *)
    echo "OS '$(uname)' not supported!" >&2
    exit 1
    ;;
  esac
}

# verifySupported checks that the os/arch combination is supported for binary builds.
verifySupported() {
  supported="Linux-x86_64\nLinux-arm64\nLinux-armv6\nLinux-armv7\nDarwin-x86_64\nDarwin-arm64\nWindows-x86_64\nWindows-arm64\nWindows-armv6\nWindows-armv7"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "No prebuild binary for ${OS}-${ARCH}."
    exit 1
  fi

  if
    ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1
  then
    echo "Either curl or wget is required"
    exit 1
  fi
}

# getDownloadURL retrieves the download URL and checksum URL.
getDownloadURL() {
  version="$(grep <"$HELM_PLUGIN_DIR/$PLUGIN_MANIFEST" "version" | cut -d '"' -f 2)"
  ext="tar.gz"
  if [ "$OS" = "Windows" ]; then
    ext="zip"
  fi
  if [ "$SCRIPT_MODE" = "install" ] && [ -n "$version" ]; then
    DOWNLOAD_URL="https://github.com/${PROJECT_GH}/releases/download/${version}/${PROJECT_NAME}_${version}_${OS}_${ARCH}.${ext}"
    CHECKSUM_URL="https://github.com/${PROJECT_GH}/releases/download/${version}/checksums.txt"
  else
    DOWNLOAD_URL="https://github.com/${PROJECT_GH}/releases/latest/download/${PROJECT_NAME}_${version}_${OS}_${ARCH}.${ext}"
    CHECKSUM_URL="https://github.com/${PROJECT_GH}/releases/latest/download/checksums.txt"
  fi
}

# Temporary dir
mkTempDir() {
  HELM_TMP="$(mktemp -d -t "${PROJECT_NAME}-XXXXXX")"
}

rmTempDir() {
  if [ -d "${HELM_TMP:-/tmp/helm-schema}" ]; then
    rm -rf "${HELM_TMP:-/tmp/helm-schema}"
  fi
}

# downloadFile downloads the latest binary package and the checksum.
downloadFile() {
  PLUGIN_TMP_FILE="${HELM_TMP}/${PROJECT_NAME}.tar.gz"
  PLUGIN_CHECKSUMS_FILE="${HELM_TMP}/${PROJECT_NAME}_checksums.txt"
  echo "Downloading ..."
  echo "$DOWNLOAD_URL"
  echo "$CHECKSUM_URL"
  if
    command -v curl >/dev/null 2>&1
  then
    curl -sSf -L "$DOWNLOAD_URL" >"$PLUGIN_TMP_FILE"
    curl -sSf -L "$CHECKSUM_URL" >"$PLUGIN_CHECKSUMS_FILE"
  elif
    command -v wget >/dev/null 2>&1
  then
    wget -q -O - "$DOWNLOAD_URL" >"$PLUGIN_TMP_FILE"
    wget -q -O - "$CHECKSUM_URL" >"$PLUGIN_CHECKSUMS_FILE"
  fi
}

validateChecksum() {
  if ! grep -q ${1} ${2}; then
    echo "Invalid checksum" >/dev/stderr
    exit 1
  fi
  echo "Checksum is valid."
}

# installFile verifies the SHA256 for the file, then unpacks and installs it.
installFile() {
  if command -v sha256sum >/dev/null 2>&1; then
    checksum=$(sha256sum ${PLUGIN_TMP_FILE} | awk '{ print $1 }')
    validateChecksum ${checksum} ${PLUGIN_CHECKSUMS_FILE}
  elif command -v openssl >/dev/null 2>&1; then
    checksum=$(openssl dgst -sha256 ${PLUGIN_TMP_FILE} | awk '{ print $2 }')
    validateChecksum ${checksum} ${PLUGIN_CHECKSUMS_FILE}
  else
    echo "WARNING: no tool found to verify checksum" >/dev/stderr
  fi

  HELM_TMP_BIN="$HELM_TMP/$BINARY_NAME"
  if [ "${OS}" = "Windows" ]; then
    HELM_TMP_BIN="$HELM_TMP_BIN.exe"
    unzip "$PLUGIN_TMP_FILE" -d "$HELM_TMP"
  else
    tar xzf "$PLUGIN_TMP_FILE" -C "$HELM_TMP"
  fi
  echo "Preparing to install into ${HELM_PLUGIN_DIR}"
  mkdir -p "$HELM_PLUGIN_DIR/bin"
  cp "$HELM_TMP_BIN" "$HELM_PLUGIN_DIR/bin"
}

# exit_trap is executed if on exit (error or not).
exit_trap() {
  result=$?
  rmTempDir
  if [ "$result" != "0" ]; then
    echo "Failed to install $PROJECT_NAME"
    printf "\tFor support, go to https://github.com/%s.\n" "$PROJECT_GH"
  fi
  exit $result
}

# testVersion tests the installed client to make sure it is working.
testVersion() {
  set +e
  echo "$PROJECT_NAME installed into $HELM_PLUGIN_DIR"
  "${HELM_PLUGIN_DIR}/bin/$BINARY_NAME" --version
  set -e
}

# Execution

#Stop execution on any error
trap "exit_trap" EXIT
set -e
initArch
initOS
verifySupported
getDownloadURL
mkTempDir
downloadFile
installFile
testVersion
