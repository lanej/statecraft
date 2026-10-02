#!/bin/sh
set -eu
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  printf 'Run make format-go; these files need gofmt:\n%s\n' "$unformatted"
  exit 1
fi
