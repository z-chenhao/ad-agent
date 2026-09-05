#!/bin/sh
set -eu

matches=$(git ls-files -co --exclude-standard -z \
  | perl -0ne 'chomp; print "$_\0" unless /\.(?:png|jpe?g|gif|webp|avif|mp4|m4v|mov|webm|ico|woff2?|ttf|zip|sqlite3?)\z/i' \
  | xargs -0 rg -I -l '[\p{Han}]' 2>/dev/null || true)
if [ -n "$matches" ]; then
  printf '%s\n' "Repository-authored files must be English. Chinese text found in:" >&2
  printf '%s\n' "$matches" >&2
  exit 1
fi

legacy_names=$(git ls-files -co --exclude-standard -z \
  | perl -0ne 'chomp; print "$_\0" unless /(?:^|\/)(?:node_modules|dist)(?:\/|$)|\.(?:png|jpe?g|gif|webp|avif|mp4|m4v|mov|webm|ico|woff2?|ttf|zip|sqlite3?)\z/i' \
  | xargs -0 rg -I -l -i 'p[o]rtfolio|single[_ -]advertiser' 2>/dev/null || true)
if [ -n "$legacy_names" ]; then
  printf '%s\n' "Legacy scope names are not allowed; use Advertiser or Manager:" >&2
  printf '%s\n' "$legacy_names" >&2
  exit 1
fi

legacy_paths=$(git ls-files -co --exclude-standard -z \
  | perl -0ne 'chomp; print "$_\n" if -e $_ && /(?:^|\/)(?:p[o]rtfolio|single[_ -]advertiser)(?:\/|\.|$)/i')
if [ -n "$legacy_paths" ]; then
  printf '%s\n' "Legacy scope names are not allowed in repository paths:" >&2
  printf '%s\n' "$legacy_paths" >&2
  exit 1
fi
