#!/bin/bash
# perf matrix: 3 prompts x 3 reps x 2 lanes, lanes interleaved. TSV: ts lane prompt rep secs exit reply_chars
OUT=$1
P[0]='hi, quick check: what is 2+2?'
P[1]='Say hello back in one short sentence.'
P[2]='In about 80 words, explain what a tidal lock is.'
for rep in 1 2 3; do for p in 0 1 2; do for lane in echo deep; do
  f=$(dirname $OUT)/raw-$lane-p$p-r$rep.json
  a=$(python3 -c 'import json,sys;print(json.dumps({"author":"perf-test","content":sys.argv[1]}))' "${P[$p]}")
  t0=$(python3 -c 'import time;print(time.time())')
  mcporter call tailnet_coilyco_sirens_$lane.turn --timeout 180000 --output json --args "$a" > $f 2>&1; rc=$?
  t1=$(python3 -c 'import time;print(time.time())')
  secs=$(python3 -c "print(round($t1-$t0,2))")
  printf "%s\t%s\tp%s\t%s\t%s\t%s\t%s\n" "$(date -u +%T)" $lane $p $rep $secs $rc $(wc -c < $f | tr -d ' ') | tee -a $OUT
done; done; done
