#!/bin/bash

basedir="$(git rev-parse --show-toplevel)"
e2edir="${basedir}/test/e2e"

find "${e2edir}" -name "filter.so" -exec dirname {} \; | while read -r dir; do
  echo "Clean up e2e filter in ${dir}"
  rm -f "${dir}/filter.so"
done
