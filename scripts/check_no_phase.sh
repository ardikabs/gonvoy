#!/usr/bin/env bash
# Guard: the phase filter runtime is retired, only the bridge remains.
# Fails if phase constructors or handler symbols appear in non-test Go sources.
set -u

matches=$(grep -rn -E "NewFilterChainFactory|NewHttpFilterFactory|HttpFilterHandler|HttpFilterAction|Composer" --include="*.go" . | grep -v "_test.go" | grep -v "^\./docs/" || true)

if [ -n "$matches" ]; then
	echo "phase symbols still present:"
	echo "$matches"
	exit 1
fi

echo "no phase symbols in non-test Go sources"
