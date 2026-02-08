#!/usr/bin/env bash
#
# Count Go source lines and spec word counts in this repository.
#

cd "${1:-$(dirname "$0")/..}" || exit 1

# Count lines in Go files, excluding tests, vendor, and magefiles
go_prod=$(find . -type f -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/magefiles/*" -print0 2>/dev/null |
  xargs -0 wc -l 2>/dev/null | tail -1 | awk '{print $1}')

# Count lines in Go test files
go_test=$(find . -type f -name "*_test.go" ! -path "*/vendor/*" -print0 2>/dev/null |
  xargs -0 wc -l 2>/dev/null | tail -1 | awk '{print $1}')

# Count words in spec files by category
spec_prd=$(cat docs/product-requirements/*.yaml 2>/dev/null | wc -w | awk '{print $1}')
spec_uc=$(cat docs/use-cases/*.yaml 2>/dev/null | wc -w | awk '{print $1}')
spec_test=$(cat docs/test-suites/*.yaml 2>/dev/null | wc -w | awk '{print $1}')

printf '{"go_loc_prod":%s,"go_loc_test":%s,"go_loc":%s,"spec_wc_prd":%s,"spec_wc_uc":%s,"spec_wc_test":%s}\n' \
  "${go_prod:-0}" "${go_test:-0}" "$((${go_prod:-0} + ${go_test:-0}))" \
  "${spec_prd:-0}" "${spec_uc:-0}" "${spec_test:-0}"
