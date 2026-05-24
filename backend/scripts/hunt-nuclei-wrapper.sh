#!/bin/sh
# HUNT_NUCLEI_SPLIT_WRAPPER_V3

REAL="${HUNT_NUCLEI_REAL:-/usr/local/bin/nuclei.real}"
LOG="${HUNT_NUCLEI_WRAPPER_LOG:-/tmp/hunt-nuclei-wrapper.log}"
BUILTIN_TIMEOUT="${HUNT_NUCLEI_BUILTIN_TIMEOUT:-300}"
SNAPSHOT_KEEP="${HUNT_NUCLEI_KEEP_SNAPSHOTS:-5}"

log() {
  msg="hunt nuclei wrapper v3: $*"
  printf '%s\n' "$msg" >&2
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)" "$msg" >> "$LOG" 2>/dev/null || true
}

q() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

append_arg() {
  var="$1"
  val="$2"
  eval "$var=\"\${$var} $(q "$val")\""
}

count_lines() {
  f="$1"
  if [ -f "$f" ]; then
    wc -l < "$f" 2>/dev/null || printf '0\n'
  else
    printf '0\n'
  fi
}

snapshot_output() {
  if [ -z "${out:-}" ] || [ ! -f "$out" ]; then
    return 0
  fi

  cp "$out" "$out.last" 2>/dev/null || true

  keep="${SNAPSHOT_KEEP:-5}"
  case "$keep" in
    ''|*[!0-9]*)
      keep=5
      ;;
  esac

  if [ "$keep" -gt 0 ]; then
    snap="$out.snapshot.$(date -u +%Y%m%dT%H%M%SZ)"
    cp "$out" "$snap" 2>/dev/null || true
    log "snapshot file=$snap last=$out.last lines=$(count_lines "$out") keep=$keep"

    dir="$(dirname "$out")"
    base="$(basename "$out")"
    ls -1t "$dir/$base".snapshot.* 2>/dev/null \
      | awk -v keep="$keep" 'NR > keep { print }' \
      | while IFS= read -r old_snapshot; do
          rm -f "$old_snapshot" 2>/dev/null || true
        done
  else
    log "snapshot disabled last=$out.last lines=$(count_lines "$out")"
  fi
}

run_eval() {
  label="$1"
  cmd="$2"
  log "$label cmd=$cmd"
  sh -c "$cmd" >> "$LOG" 2>&1
  rc=$?
  log "$label rc=$rc"
  return "$rc"
}

run_builtin() {
  label="$1"
  cmd="$2"

  if [ "${BUILTIN_TIMEOUT:-0}" != "0" ] && command -v timeout >/dev/null 2>&1; then
    run_eval "$label" "timeout ${BUILTIN_TIMEOUT}s $cmd"
  else
    run_eval "$label" "$cmd"
  fi
}

if [ ! -x "$REAL" ]; then
  log "real nuclei binary is missing or not executable: $REAL"
  exit 127
fi

out=""
common_args=""
builtin_tpl_args=""
custom_tpl_args=""
tag_args=""
builtin_count=0
custom_count=0

while [ "$#" -gt 0 ]; do
  a="$1"
  shift

  case "$a" in
    -o|-output)
      if [ "$#" -gt 0 ]; then
        out="$1"
        shift
      else
        append_arg common_args "$a"
      fi
      ;;

    -t|-template|-templates)
      if [ "$#" -gt 0 ]; then
        v="$1"
        shift

        case "$v" in
          /data/nuclei/custom/*|*/data/nuclei/custom/*|*/custom/users/*)
            append_arg custom_tpl_args "$a"
            append_arg custom_tpl_args "$v"
            custom_count=$((custom_count + 1))
            ;;
          *)
            append_arg builtin_tpl_args "$a"
            append_arg builtin_tpl_args "$v"
            builtin_count=$((builtin_count + 1))
            ;;
        esac
      else
        append_arg common_args "$a"
      fi
      ;;

    -tags|-tag|-include-tags)
      if [ "$#" -gt 0 ]; then
        v="$1"
        shift
        append_arg tag_args "$a"
        append_arg tag_args "$v"
      else
        append_arg tag_args "$a"
      fi
      ;;

    *)
      append_arg common_args "$a"
      ;;
  esac
done

[ -n "$out" ] || out="/tmp/hunt-nuclei-wrapper-$$.jsonl"
mkdir -p "$(dirname "$out")" 2>/dev/null || true
rm -f "$out"

log "split enabled custom_templates=$custom_count builtin_templates=$builtin_count out=$out real=$REAL"

if [ "$custom_count" -gt 0 ] && [ "$builtin_count" -gt 0 ]; then
  custom_out="${out}.$$.custom"
  builtin_out="${out}.$$.builtin"
  rm -f "$custom_out" "$builtin_out"

  custom_cmd="$(q "$REAL") $common_args $custom_tpl_args -o $(q "$custom_out")"
  builtin_cmd="$(q "$REAL") $common_args $builtin_tpl_args $tag_args -o $(q "$builtin_out")"

  log "running custom nuclei without -tags"
  run_eval "custom" "$custom_cmd" || true
  log "custom output lines=$(count_lines "$custom_out") file=$custom_out"

  log "running builtin nuclei with original -tags timeout=${BUILTIN_TIMEOUT}s"
  run_builtin "builtin" "$builtin_cmd" || true
  log "builtin output lines=$(count_lines "$builtin_out") file=$builtin_out"

  cat "$custom_out" "$builtin_out" > "$out" 2>/dev/null || true
  rm -f "$custom_out" "$builtin_out"

  log "merged output lines=$(count_lines "$out") file=$out"
  snapshot_output
  exit 0
fi

if [ "$custom_count" -gt 0 ]; then
  cmd="$(q "$REAL") $common_args $custom_tpl_args -o $(q "$out")"
  log "running custom-only nuclei without -tags"
  run_eval "custom-only" "$cmd" || true
  log "output lines=$(count_lines "$out") file=$out"
  snapshot_output
  exit 0
fi

if [ "$builtin_count" -gt 0 ]; then
  cmd="$(q "$REAL") $common_args $builtin_tpl_args $tag_args -o $(q "$out")"
  log "running builtin-only nuclei with original -tags timeout=${BUILTIN_TIMEOUT}s"
  run_builtin "builtin-only" "$cmd" || true
  log "output lines=$(count_lines "$out") file=$out"
  snapshot_output
  exit 0
fi

cmd="$(q "$REAL") $common_args $tag_args -o $(q "$out")"
log "running passthrough nuclei"
run_eval "passthrough" "$cmd" || true
log "output lines=$(count_lines "$out") file=$out"
snapshot_output
exit 0
