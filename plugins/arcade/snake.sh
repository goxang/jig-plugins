#!/usr/bin/env bash
# Snake for a terminal pane: arrows, WASD or hjkl steer, p pauses, r restarts, q quits.
# ponytail: needs bash 4 for fractional read timeouts (macOS: brew install bash).
set -u

read -r rows cols < <(stty size 2>/dev/null || echo "24 80")
W=$(( (cols - 2) / 2 )) H=$(( rows - 3 ))
(( W > 40 )) && W=40
(( H > 20 )) && H=20

cleanup() { printf '\e[?25h\e[0m\e[2J\e[H'; stty sane; }
trap cleanup EXIT
stty -echo -icanon
printf '\e[?25l\e[2J'

reset() {
  sx=($((W/2-3)) $((W/2-2)) $((W/2-1)) $((W/2))) sy=($((H/2)) $((H/2)) $((H/2)) $((H/2)))
  dx=1 dy=0 score=0 over=0 paused=0
  place
}

on_snake() { # x y
  local i
  for ((i = 0; i < ${#sx[@]}; i++)); do
    (( sx[i] == $1 && sy[i] == $2 )) && return 0
  done
  return 1
}

place() {
  while :; do
    fx=$((RANDOM % W)) fy=$((RANDOM % H))
    on_snake "$fx" "$fy" || return
  done
}

turn() { # dx dy, ignoring a reversal into itself
  (( $1 == -dx && $2 == -dy )) || { ndx=$1 ndy=$2; }
}

step() {
  dx=$ndx dy=$ndy
  local hx=$(( sx[${#sx[@]}-1] + dx )) hy=$(( sy[${#sy[@]}-1] + dy ))
  if (( hx < 0 || hy < 0 || hx >= W || hy >= H )) || on_snake "$hx" "$hy"; then
    over=1
    return
  fi
  sx+=("$hx") sy+=("$hy")
  if (( hx == fx && hy == fy )); then
    score=$((score + 1))
    place
  else
    sx=("${sx[@]:1}") sy=("${sy[@]:1}")
  fi
}

draw() {
  local x y line out status="snake  score $score   arrows/wasd/hjkl · p pause · r restart · q quit"
  (( over )) && status="game over, score $score   r restart · q quit"
  (( paused )) && status="paused   p resume"
  local -a grid=()
  for ((y = 0; y < H; y++)); do grid[y]=$(printf '%*s' $((W * 2)) ''); done
  put() { grid[$2]="${grid[$2]:0:$(($1*2))}$3${grid[$2]:$(($1*2+2))}"; }
  put "$fx" "$fy" "()"
  for ((x = 0; x < ${#sx[@]}; x++)); do put "${sx[x]}" "${sy[x]}" "██"; done
  out=$'\e[H\e[7m'"$(printf '%-*s' $((W * 2 + 2)) "$status")"$'\e[0m\n┌'
  line=$(printf '%*s' $((W * 2)) '')
  out+="${line// /─}┐"$'\n'
  for ((y = 0; y < H; y++)); do
    line=${grid[y]//()/$'\e[31m()\e[32m'}
    out+="│"$'\e[32m'"$line"$'\e[0m│\n'
  done
  line=$(printf '%*s' $((W * 2)) '')
  out+="└${line// /─}┘"
  printf '%s' "$out"
}

reset
ndx=$dx ndy=$dy
while :; do
  draw
  key=""
  IFS= read -rsn1 -t 0.12 key
  if [[ $key == $'\e' ]]; then
    IFS= read -rsn2 -t 0.01 rest
    key+=$rest
  fi
  case $key in
    $'\e[A' | w | k) turn 0 -1 ;;
    $'\e[B' | s | j) turn 0 1 ;;
    $'\e[C' | d | l) turn 1 0 ;;
    $'\e[D' | a | h) turn -1 0 ;;
    p) paused=$((!paused)) ;;
    r) reset; ndx=$dx ndy=$dy ;;
    q) exit 0 ;;
  esac
  (( over || paused )) || step
done
