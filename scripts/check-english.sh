#!/bin/sh
set -eu

matches=$(git ls-files -co --exclude-standard -z \
  | perl -0ne 'chomp; print "$_\0" unless /\.(?:png|jpe?g|gif|webp|ico|woff2?|ttf|zip|sqlite3?)\z/i' \
  | xargs -0 rg -I -l '[\p{Han}]' 2>/dev/null || true)
if [ -n "$matches" ]; then
  printf '%s\n' "Repository-authored files must be English. Chinese text found in:" >&2
  printf '%s\n' "$matches" >&2
  exit 1
fi
