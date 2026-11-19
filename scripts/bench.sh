#!/usr/bin/env bash
# Run the baseline benchmark set and compare runs with benchstat.
#
#   scripts/bench.sh run [label]         run the set, write
#                                        memory-bank/reports/bench-<label>-<date>.txt
#   scripts/bench.sh compare OLD NEW     benchstat OLD NEW
#
# Environment overrides:
#   COUNT      samples per benchmark (default 10; benchstat needs several)
#   BENCHTIME  per-sample time        (default 200ms)
#   PATTERN    -bench regexp          (default 'Baseline', the curated set)
#   TIMEOUT    go test timeout        (default 90m)
#
# For trustworthy numbers: close other heavy programs, plug in the laptop, and
# compare runs taken with the same toolchain on the same machine. The header of
# each report records the toolchain, CPU, commit and whether the tree was dirty.
set -euo pipefail

cd "$(dirname "$0")/.."

COUNT="${COUNT:-10}"
BENCHTIME="${BENCHTIME:-200ms}"
PATTERN="${PATTERN:-Baseline}"
TIMEOUT="${TIMEOUT:-90m}"

cmd="${1:-run}"

case "$cmd" in
run)
	label="${2:-baseline}"
	outdir="memory-bank/reports"
	out="$outdir/bench-${label}-$(date +%Y-%m-%d).txt"
	mkdir -p "$outdir"
	{
		# "key: value" lines are benchfmt configuration; benchstat records them.
		echo "toolchain: $(go env GOVERSION)"
		echo "module-go: $(go list -m -f '{{.GoVersion}}')"
		echo "commit: $(git rev-parse --short HEAD)"
		echo "dirty-files: $(git status --porcelain | wc -l | tr -d ' ')"
		echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
		echo "load: $(uptime | sed 's/.*load averages*: //')"
		echo "bench-pattern: ${PATTERN}"
		echo "bench-count: ${COUNT}"
		echo "bench-time: ${BENCHTIME}"
		go test -run '^$' -bench "$PATTERN" -benchmem -count "$COUNT" \
			-benchtime "$BENCHTIME" -timeout "$TIMEOUT" ./...
	} 2>&1 | tee "$out"
	echo
	echo "wrote $out"
	;;
compare)
	if [ "$#" -ne 3 ]; then
		echo "usage: $0 compare OLD NEW" >&2
		exit 2
	fi
	if ! command -v benchstat >/dev/null 2>&1; then
		echo "benchstat not found: go install golang.org/x/perf/cmd/benchstat@latest" >&2
		exit 1
	fi
	benchstat "$2" "$3"
	;;
*)
	echo "usage: $0 {run [label] | compare OLD NEW}" >&2
	exit 2
	;;
esac
