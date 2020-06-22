#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh

follow_test() {
  i=$1
  j=$2

  k $j label node --all a=b
  k $i apply -f test/e2e/follow/test.yaml
  k $i wait job/follow --for=condition=Complete
  k $i delete -f test/e2e/follow/test.yaml
  k $j label node --all a-
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  follow_test "${@}"
fi
