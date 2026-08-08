#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "${PROJECT_ROOT:-$PWD}" && pwd)"
PROOFS_DIR="${REPO_ROOT}/engine/Proofs"
OUT_PATH="${1:-${REPO_ROOT}/docs/proofstats.md}"
PROFILE="${PROOFSTATS_PROFILE:-auto}"

shopt -s nullglob
files=("${PROOFS_DIR}"/*.lean)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "no proof files found under ${PROOFS_DIR}" >&2
  exit 1
fi

tmp_rows="$(mktemp)"
trap 'rm -f "${tmp_rows}" "${tmp_summary}" "${tmp_categories}" "${tmp_files}"' EXIT
tmp_summary="$(mktemp)"
tmp_categories="$(mktemp)"
tmp_files="$(mktemp)"

detect_profile() {
  if [[ "${PROFILE}" != "auto" ]]; then
    printf '%s' "${PROFILE}"
    return
  fi

  if [[ -f "${PROOFS_DIR}/ProcedureShape.lean" ]]; then
    printf 'arb'
    return
  fi

  if [[ -f "${PROOFS_DIR}/Contempt.lean" ]]; then
    printf 'adc'
    return
  fi

  printf 'generic'
}

PROFILE="$(detect_profile)"

category_for() {
  local base="$1"
  case "${PROFILE}" in
    adc)
      case "$base" in
        Contempt.lean|Jury.lean|Party.lean)
          printf 'Domain'
          ;;
        StatusBasics.lean|OverrideBasics.lean|LimitBasics.lean|EffectiveLimitBasics.lean|TimeBasics.lean)
          printf 'Basics'
          ;;
        Validate*.lean)
          printf 'Validation'
          ;;
        *)
          printf 'Flow/Invariants'
          ;;
      esac
      ;;
    arb)
      case "$base" in
        Samples.lean|Reachability.lean|InitializeCase.lean)
          printf 'Foundations'
          ;;
        MeritsFlow.lean|Deliberation.lean|StepPreservation.lean)
          printf 'Execution'
          ;;
        ProcedureShape.lean|AggregateLimits.lean|GlobalInvariants.lean|ReachableInvariants.lean|ReachableMaterialLimits.lean|CaseFrame.lean|CouncilIntegrity.lean|CouncilStatus.lean|RecordProvenance.lean)
          printf 'Invariants'
          ;;
        OutcomeSoundness.lean|NoStuck.lean|BoundedTermination.lean)
          printf 'Results'
          ;;
        *)
          printf 'Other'
          ;;
      esac
      ;;
    *)
      printf 'Other'
      ;;
  esac
}

count_proofs() {
  awk '
    BEGIN { in_block = 0 }
    {
      line = $0
      while (1) {
        if (in_block) {
          end = index(line, "-/")
          if (end == 0) {
            line = ""
            break
          }
          line = substr(line, end + 2)
          in_block = 0
        }

        block = index(line, "/-")
        dash = index(line, "--")

        if (dash > 0 && (block == 0 || dash < block)) {
          line = substr(line, 1, dash - 1)
          block = 0
        }

        if (block == 0) {
          break
        }

        prefix = substr(line, 1, block - 1)
        suffix = substr(line, block + 2)
        end = index(suffix, "-/")
        if (end == 0) {
          line = prefix
          in_block = 1
          break
        }
        line = prefix substr(suffix, end + 2)
      }

      if (line ~ /^[[:space:]]*(theorem|lemma)[[:space:]]+/) {
        c++
      }
    }
    END { print c + 0 }
  ' "$1"
}

for path in "${files[@]}"; do
  base="$(basename "$path")"
  category="$(category_for "$base")"
  proofs="$(count_proofs "$path")"
  lines="$(wc -l < "$path" | tr -d ' ')"
  bytes="$(wc -c < "$path" | tr -d ' ')"
  printf '%s\t%s\t%s\t%s\t%s\n' "$category" "$base" "$proofs" "$lines" "$bytes" >> "$tmp_rows"
done

awk -F'\t' '
  { files += 1; proofs += $3; lines += $4; bytes += $5 }
  END {
    printf "Proof files\t%d\n", files
    printf "Total proofs\t%d\n", proofs
    printf "Total lines\t%d\n", lines
    printf "Total bytes\t%d\n", bytes
  }
' "$tmp_rows" > "$tmp_summary"

awk -F'\t' '
  {
    cat = $1
    files[cat] += 1
    proofs[cat] += $3
    lines[cat] += $4
    bytes[cat] += $5
  }
  END {
    for (cat in files) {
      printf "%s\t%d\t%d\t%d\t%d\n", cat, files[cat], proofs[cat], lines[cat], bytes[cat]
    }
  }
' "$tmp_rows" | sort -t$'\t' -k1,1 > "$tmp_categories"

sort -t$'\t' -k1,1 -k2,2 "$tmp_rows" > "$tmp_files"

{
  printf '# Proof Stats\n\n'
  printf 'Generated from `engine/Proofs/*.lean` using declarations matching `^(theorem|lemma)`.\n\n'

  printf '## Summary\n\n'
  printf '| Metric | Value |\n'
  printf '|---|---:|\n'
  while IFS=$'\t' read -r metric value; do
    printf '| %s | %s |\n' "$metric" "$value"
  done < "$tmp_summary"

  printf '\n## By Category\n\n'
  printf '| Category | Files | Proofs | Lines | Bytes |\n'
  printf '|---|---:|---:|---:|---:|\n'
  while IFS=$'\t' read -r category files proofs lines bytes; do
    printf '| %s | %s | %s | %s | %s |\n' "$category" "$files" "$proofs" "$lines" "$bytes"
  done < "$tmp_categories"

  printf '\n## By File\n\n'
  printf '| Category | File | Proofs | Lines | Bytes |\n'
  printf '|---|---|---:|---:|---:|\n'
  while IFS=$'\t' read -r category file proofs lines bytes; do
    printf '| %s | %s | %s | %s | %s |\n' "$category" "$file" "$proofs" "$lines" "$bytes"
  done < "$tmp_files"
} > "$OUT_PATH"

echo "wrote ${OUT_PATH}"
