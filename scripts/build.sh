#!/usr/bin/env bash
#
# Build, test, and lint automation for crumbs.
#
# Usage: ./scripts/build.sh <command> [options]
#
# Commands:
#   build           Compile cupboard binary to bin/
#   test            Run all tests (unit + integration)
#   test-unit       Run only unit tests (exclude integration)
#   test-integration Run only integration tests (builds first)
#   lint            Run golangci-lint
#   clean           Remove build artifacts
#   install         Install cupboard to GOPATH/bin
#   help            Show this message
#
# Options:
#   VERBOSE=1       Enable verbose test output

set -e

cd "${BASH_SOURCE[0]%/*}/.." || exit 1

BINARY_NAME="cupboard"
BINARY_DIR="bin"
CMD_DIR="./cmd/cupboard"

GO_BUILD_FLAGS="-v"
GO_TEST_FLAGS=""
[ "${VERBOSE:-}" = "1" ] && GO_TEST_FLAGS="-v"

cmd_build() {
  mkdir -p "$BINARY_DIR"
  go build $GO_BUILD_FLAGS -o "$BINARY_DIR/$BINARY_NAME" "$CMD_DIR"
}

cmd_test() {
  go test $GO_TEST_FLAGS ./...
}

cmd_test_unit() {
  go test $GO_TEST_FLAGS $(go list ./... | grep -v /tests/)
}

cmd_test_integration() {
  cmd_build
  go test $GO_TEST_FLAGS ./tests/...
}

cmd_lint() {
  golangci-lint run ./...
}

cmd_clean() {
  rm -rf "$BINARY_DIR"
  go clean
}

cmd_install() {
  cmd_build
  cp "$BINARY_DIR/$BINARY_NAME" "$(go env GOPATH)/bin/$BINARY_NAME"
}

cmd_help() {
  sed -n '2,/^[^#]/{ /^#/s/^# \{0,1\}//p; }' "${BASH_SOURCE[0]}"
}

case "${1:-help}" in
  build)            cmd_build ;;
  test)             cmd_test ;;
  test-unit)        cmd_test_unit ;;
  test-integration) cmd_test_integration ;;
  lint)             cmd_lint ;;
  clean)            cmd_clean ;;
  install)          cmd_install ;;
  help)             cmd_help ;;
  *)
    echo "Unknown command: $1" >&2
    cmd_help >&2
    exit 1
    ;;
esac
