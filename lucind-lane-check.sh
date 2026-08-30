#!/bin/sh
# lucind-lane-check.sh
#
# Deterministic mechanical self-check for a lucind-ai document lane (a lens
# or synthesis packet in a planning fan-out). Every profiled fan-out lane
# spent 1-3 minutes at the end narrating mechanical facts in prose: word
# counts, `git status --porcelain`, restating done-criteria, `.lucind/
# result.json` bookkeeping. This script performs those checks and prints one
# compact PASS/FAIL report the lane pastes into its result envelope instead
# of re-deriving and narrating each fact by hand.
#
# It also covers a second, cheaper property: that every `file:line` citation
# a lens lane names in its own `## Citation Manifest` at least resolves to a
# real location before the lane commits. This is an existence/grep-level
# check only -- does the file exist, are the cited line(s) within it, and
# (for a range) is the start line not after the end line -- never a claim
# that the citation supports what the draft says it supports. That semantic
# re-verification stays the synthesizer's job.
#
# This script judges NOTHING about whether the lane's actual work is good.
# It does not read the packet's done-criteria and does not decide `status`.
# The lane remains the one that judges its own done-criteria; this only
# removes the busywork of narrating facts a script checks faster and more
# reliably than prose.
#
# Usage:
#   ./lucind-lane-check.sh --file <path>
#       [--budget <n>] [--exclude-section "<heading>"]...
#       [--require-section "<heading>"]...
#       [--verify-citations]
#       [--skip-git] [--result <path>] [--skip-result]
#
# --budget <n>            Word count under <n> fails the file. Word count is
#                          a plain whitespace split (wc -w), the same
#                          approximation a lane would use by eye.
# --exclude-section NAME  Drop the "## NAME" section (heading through the
#                          next "## " heading or EOF) before counting words.
#                          Repeatable. Matches only "## " (H2) headings.
# --require-section NAME  Fail unless a line reading exactly "## NAME"
#                          exists in --file. Repeatable.
# --verify-citations      Parse --file's "## Citation Manifest" table and,
#                          for each `path:line` or `path:line-line` citation,
#                          check the file exists and has enough lines to
#                          contain the WHOLE cited range -- both start and
#                          end, not just start. A start line after the end
#                          line (e.g. `path:40-12`) fails as malformed. An en
#                          dash (U+2013) range separator, seen in real
#                          model-authored manifests, is normalized to a
#                          hyphen before parsing so it is validated the same
#                          way, not silently skipped. Existence/bounds only
#                          -- never opens the claim.
# --skip-git              Skip the `git status --porcelain` cleanliness
#                          check (e.g. when checking content before the
#                          lane's commit).
# --result <path>         Result envelope path (default: .lucind/result.json).
# --skip-result           Skip the result-envelope existence/parse check
#                          (e.g. before the lane has written it yet).
#
# Exit status: 0 if every requested check passed, 1 if any failed. Prints
# the full report regardless of exit status -- this is a report tool, not a
# fail-fast one.

set -u

usage() {
  cat <<'EOF'
usage: lucind-lane-check.sh --file <path>
         [--budget <n>] [--exclude-section "<heading>"]...
         [--require-section "<heading>"]... [--verify-citations]
         [--skip-git] [--result <path>] [--skip-result]
EOF
}

file=""
budget=""
result_path=".lucind/result.json"
skip_git=0
skip_result=0
verify_citations=0
exclude_file=$(mktemp)
require_file=$(mktemp)
trap 'rm -f "$exclude_file" "$require_file"' EXIT

while [ $# -gt 0 ]; do
  case "$1" in
    --file) file="$2"; shift 2 ;;
    --budget) budget="$2"; shift 2 ;;
    --exclude-section) printf '%s\n' "$2" >> "$exclude_file"; shift 2 ;;
    --require-section) printf '%s\n' "$2" >> "$require_file"; shift 2 ;;
    --verify-citations) verify_citations=1; shift ;;
    --skip-git) skip_git=1; shift ;;
    --result) result_path="$2"; shift 2 ;;
    --skip-result) skip_result=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "lucind-lane-check.sh: unrecognized argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ -z "$file" ]; then
  echo "lucind-lane-check.sh: --file is required" >&2
  usage >&2
  exit 2
fi

fail=0

echo "lucind-lane-check.sh report for $file"
echo "----------------------------------------"

# 1. Word count against budget, excluding any --exclude-section content.
if [ -n "$budget" ]; then
  if [ ! -f "$file" ]; then
    echo "FAIL  word budget: $file does not exist"
    fail=1
  else
    working="$file"
    scratch=""
    while IFS= read -r heading; do
      [ -n "$heading" ] || continue
      next_scratch=$(mktemp)
      awk -v heading="$heading" '
        BEGIN { skip = 0 }
        /^## / {
          line = $0
          sub(/^## /, "", line)
          if (skip == 1) skip = 0
          if (line == heading) { skip = 1; next }
        }
        skip == 0 { print }
      ' "$working" > "$next_scratch"
      [ -n "$scratch" ] && rm -f "$scratch"
      scratch="$next_scratch"
      working="$scratch"
    done < "$exclude_file"

    count=$(wc -w < "$working" | tr -d '[:space:]')
    [ -n "$scratch" ] && rm -f "$scratch"

    if [ "$count" -lt "$budget" ]; then
      echo "PASS  word budget: $count / $budget words"
    else
      echo "FAIL  word budget: $count / $budget words (at or over budget)"
      fail=1
    fi
  fi
fi

# 2. Required sections present.
if [ -s "$require_file" ]; then
  if [ ! -f "$file" ]; then
    echo "FAIL  required sections: $file does not exist"
    fail=1
  else
    while IFS= read -r heading; do
      [ -n "$heading" ] || continue
      if grep -qxF "## $heading" "$file"; then
        echo "PASS  section present: ## $heading"
      else
        echo "FAIL  section missing: ## $heading"
        fail=1
      fi
    done < "$require_file"
  fi
fi

# 3. Citation existence check over "## Citation Manifest".
if [ "$verify_citations" -eq 1 ]; then
  if [ ! -f "$file" ]; then
    echo "FAIL  citation manifest: $file does not exist"
    fail=1
  else
    in_section=0
    seen_any=0
    while IFS= read -r line; do
      case "$line" in
        "## Citation Manifest")
          in_section=1
          continue
          ;;
        "## "*)
          in_section=0
          ;;
      esac
      [ "$in_section" -eq 1 ] || continue

      citation=$(printf '%s\n' "$line" | grep -oE '`[^`]+`' | head -n1 | tr -d '`')
      [ -n "$citation" ] || continue
      case "$citation" in
        *:*) : ;;
        *) continue ;;
      esac

      seen_any=1
      path="${citation%%:*}"
      linespec="${citation#*:}"

      # Some model-authored manifests use an en dash (U+2013) as the range
      # separator instead of a hyphen. Normalize it to a hyphen before
      # parsing so a range like "12–34" is validated exactly like "12-34" --
      # otherwise it would silently escape range-end validation through a
      # different delimiter than the one this check exists to close.
      linespec=$(printf '%s' "$linespec" | sed 's/–/-/')

      case "$linespec" in
        *-*)
          startline="${linespec%%-*}"
          endline="${linespec#*-}"
          ;;
        *)
          startline="$linespec"
          endline="$linespec"
          ;;
      esac

      case "$startline" in
        ''|*[!0-9]*)
          echo "SKIP  citation $citation: could not parse a line number"
          continue
          ;;
      esac
      case "$endline" in
        ''|*[!0-9]*)
          echo "SKIP  citation $citation: could not parse the end of the line range"
          continue
          ;;
      esac

      if [ "$startline" -gt "$endline" ]; then
        echo "FAIL  citation $citation: malformed range, start line $startline is after end line $endline"
        fail=1
        continue
      fi

      if [ ! -f "$path" ]; then
        echo "FAIL  citation $citation: file does not exist"
        fail=1
        continue
      fi

      total_lines=$(wc -l < "$path" | tr -d '[:space:]')
      if [ "$endline" -le "$total_lines" ]; then
        echo "PASS  citation $citation: file exists, has $total_lines lines"
      else
        echo "FAIL  citation $citation: file has $total_lines lines, cited range ends at line $endline which is out of range"
        fail=1
      fi
    done < "$file"

    if [ "$seen_any" -eq 0 ]; then
      echo "FAIL  citation manifest: no '## Citation Manifest' section with parseable citations found"
      fail=1
    fi
  fi
fi

# 4. git status --porcelain cleanliness.
if [ "$skip_git" -eq 0 ]; then
  porcelain=$(git status --porcelain 2>&1)
  if [ -z "$porcelain" ]; then
    echo "PASS  git status --porcelain: clean"
  else
    echo "FAIL  git status --porcelain: not clean"
    echo "$porcelain" | sed 's/^/        /'
    fail=1
  fi
fi

# 5. Result envelope exists and parses as JSON.
if [ "$skip_result" -eq 0 ]; then
  if [ ! -f "$result_path" ]; then
    echo "FAIL  result envelope: $result_path not found"
    fail=1
  elif command -v python3 >/dev/null 2>&1; then
    if python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$result_path" >/dev/null 2>&1; then
      echo "PASS  result envelope: $result_path exists and parses as JSON"
    else
      echo "FAIL  result envelope: $result_path exists but does not parse as JSON"
      fail=1
    fi
  elif command -v node >/dev/null 2>&1; then
    if node -e "JSON.parse(require('fs').readFileSync(process.argv[1],'utf8'))" "$result_path" >/dev/null 2>&1; then
      echo "PASS  result envelope: $result_path exists and parses as JSON"
    else
      echo "FAIL  result envelope: $result_path exists but does not parse as JSON"
      fail=1
    fi
  else
    echo "SKIP  result envelope: $result_path exists, but no python3 or node available to verify it parses"
  fi
fi

echo "----------------------------------------"
if [ "$fail" -eq 0 ]; then
  echo "RESULT: PASS"
else
  echo "RESULT: FAIL"
fi

exit "$fail"
