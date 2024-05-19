#!/bin/bash

git config --global --add safe.directory /app

basedir="$(git rev-parse --show-toplevel)"
e2edir="${basedir}/test/e2e/filters"

find "${e2edir}" -name "main.go" -exec dirname {} \; | while read -r dir; do
  echo "Building e2e filter in ${dir}"
  pushd "${dir}" >/dev/null || exit 1
  bash -c "go build -o filter.so -buildmode=c-shared *.go"
  popd >/dev/null || exit 1
done
