#!/usr/bin/env bash
# Tic-Tac-Toe for a terminal pane: arrows/wasd/hjkl/1-9 steer, space/enter places, m mode, r restarts, q quits.
set -u

cleanup() { printf '\e[?25h\e[0m\e[2J\e[H'; stty sane 2>/dev/null || true; }
trap cleanup EXIT
stty -echo -icanon 2>/dev/null || true
printf '\e[?25l\e[2J'

board=(" " " " " " " " " " " " " " " " " ")
cursor=4
turn="X"
mode="cpu" # "cpu" or "2p"
over=0
winner=""
win_line=()
score_x=0
score_o=0
score_draw=0

reset_board() {
  board=(" " " " " " " " " " " " " " " " " ")
  cursor=4
  turn="X"
  over=0
  winner=""
  win_line=()
}

check_winner() {
  local wins=(
    "0 1 2" "3 4 5" "6 7 8"
    "0 3 6" "1 4 7" "2 5 8"
    "0 4 8" "2 4 6"
  )
  for combo in "${wins[@]}"; do
    set -- $combo
    local a=$1 b=$2 c=$3
    if [[ "${board[a]}" != " " && "${board[a]}" == "${board[b]}" && "${board[b]}" == "${board[c]}" ]]; then
      winner="${board[a]}"
      win_line=($a $b $c)
      over=1
      return
    fi
  done

  local full=1
  for cell in "${board[@]}"; do
    if [[ "$cell" == " " ]]; then
      full=0
      break
    fi
  done
  if (( full )); then
    winner="draw"
    over=1
  fi
}

cpu_move() {
  local wins=(
    "0 1 2" "3 4 5" "6 7 8"
    "0 3 6" "1 4 7" "2 5 8"
    "0 4 8" "2 4 6"
  )
  # 1. Win
  for combo in "${wins[@]}"; do
    set -- $combo
    local a=$1 b=$2 c=$3
    if [[ "${board[a]}" == "O" && "${board[b]}" == "O" && "${board[c]}" == " " ]]; then board[c]="O"; return; fi
    if [[ "${board[a]}" == "O" && "${board[c]}" == "O" && "${board[b]}" == " " ]]; then board[b]="O"; return; fi
    if [[ "${board[b]}" == "O" && "${board[c]}" == "O" && "${board[a]}" == " " ]]; then board[a]="O"; return; fi
  done

  # 2. Block
  for combo in "${wins[@]}"; do
    set -- $combo
    local a=$1 b=$2 c=$3
    if [[ "${board[a]}" == "X" && "${board[b]}" == "X" && "${board[c]}" == " " ]]; then board[c]="O"; return; fi
    if [[ "${board[a]}" == "X" && "${board[c]}" == "X" && "${board[b]}" == " " ]]; then board[b]="O"; return; fi
    if [[ "${board[b]}" == "X" && "${board[c]}" == "X" && "${board[a]}" == " " ]]; then board[a]="O"; return; fi
  done

  # 3. Center
  if [[ "${board[4]}" == " " ]]; then
    board[4]="O"
    return
  fi

  # 4. Corners
  local corners=(0 2 6 8)
  local free_corners=()
  for c in "${corners[@]}"; do
    [[ "${board[c]}" == " " ]] && free_corners+=("$c")
  done
  if (( ${#free_corners[@]} > 0 )); then
    local pick=$(( RANDOM % ${#free_corners[@]} ))
    board[${free_corners[pick]}]="O"
    return
  fi

  # 5. Edges
  local free_all=()
  for i in {0..8}; do
    [[ "${board[i]}" == " " ]] && free_all+=("$i")
  done
  if (( ${#free_all[@]} > 0 )); then
    local pick=$(( RANDOM % ${#free_all[@]} ))
    board[${free_all[pick]}]="O"
    return
  fi
}

draw() {
  local status
  if (( over )); then
    if [[ "$winner" == "draw" ]]; then
      status="game over: draw!   r restart · m mode · q quit"
    else
      status="game over: $winner wins!   r restart · m mode · q quit"
    fi
  else
    local who="Player"
    [[ "$mode" == "cpu" && "$turn" == "O" ]] && who="CPU"
    status="turn: $turn ($who)   arrows/wasd/1-9 move · space/enter place · m mode · r reset · q quit"
  fi

  local mode_str="vs CPU"
  [[ "$mode" == "2p" ]] && mode_str="2-Player"

  local W=46
  local out=$'\e[H\e[7m'"$(printf '%-*s' $W " Tic-Tac-Toe  [$mode_str]  X: $score_x  O: $score_o  Ties: $score_draw")"$'\e[0m\n'
  out+=$'\e[2m'"$(printf '%-*s' $W " $status")"$'\e[0m\n\n'

  is_win_cell() {
    local idx=$1
    for w in "${win_line[@]}"; do
      (( w == idx )) && return 0
    done
    return 1
  }

  render_cell() {
    local idx=$1
    local val="${board[idx]}"
    local is_cursor=0
    (( idx == cursor )) && is_cursor=1

    local colored_val="$val"
    if is_win_cell "$idx"; then
      colored_val=$'\e[1;42;37m'"$val"$'\e[0m'
    elif [[ "$val" == "X" ]]; then
      colored_val=$'\e[36;1mX\e[0m'
    elif [[ "$val" == "O" ]]; then
      colored_val=$'\e[33;1mO\e[0m'
    else
      if (( !over )) && (( is_cursor )); then
        colored_val=$'\e[2m·\e[0m'
      else
        colored_val=" "
      fi
    fi

    if (( is_cursor )); then
      printf '\e[7m %s \e[0m' "$colored_val"
    else
      printf ' %s ' "$colored_val"
    fi
  }

  local c0=$(render_cell 0)
  local c1=$(render_cell 1)
  local c2=$(render_cell 2)
  local c3=$(render_cell 3)
  local c4=$(render_cell 4)
  local c5=$(render_cell 5)
  local c6=$(render_cell 6)
  local c7=$(render_cell 7)
  local c8=$(render_cell 8)

  out+="       ┌─────┬─────┬─────┐"$'\n'
  out+="       │ $c0 │ $c1 │ $c2 │"$'\n'
  out+="       ├─────┼─────┼─────┤"$'\n'
  out+="       │ $c3 │ $c4 │ $c5 │"$'\n'
  out+="       ├─────┼─────┼─────┤"$'\n'
  out+="       │ $c6 │ $c7 │ $c8 │"$'\n'
  out+="       └─────┴─────┴─────┘"$'\n\n'
  out+=$'\e[2m'"   arrows/wasd/hjkl or 1-9 to move · space/enter to place · r restart"$'\e[0m\n'

  printf '%s' "$out"
}

make_move() {
  local idx=$1
  if (( over )) || [[ "${board[idx]}" != " " ]]; then
    return
  fi

  board[idx]="$turn"
  check_winner
  if (( over )); then
    if [[ "$winner" == "X" ]]; then ((score_x++)); fi
    if [[ "$winner" == "O" ]]; then ((score_o++)); fi
    if [[ "$winner" == "draw" ]]; then ((score_draw++)); fi
    return
  fi

  if [[ "$mode" == "cpu" ]]; then
    turn="O"
    draw
    sleep 0.15
    cpu_move
    check_winner
    if (( over )); then
      if [[ "$winner" == "X" ]]; then ((score_x++)); fi
      if [[ "$winner" == "O" ]]; then ((score_o++)); fi
      if [[ "$winner" == "draw" ]]; then ((score_draw++)); fi
    else
      turn="X"
    fi
  else
    if [[ "$turn" == "X" ]]; then turn="O"; else turn="X"; fi
  fi
}

reset_board

while :; do
  draw
  key=""
  IFS= read -rsn1 key
  if [[ $key == $'\e' ]]; then
    IFS= read -rsn2 -t 0.01 rest
    key+=$rest
  fi

  case "$key" in
    $'\e[A' | w | k) (( cursor >= 3 )) && cursor=$((cursor - 3)) ;;
    $'\e[B' | s | j) (( cursor <= 5 )) && cursor=$((cursor + 3)) ;;
    $'\e[D' | a | h) (( cursor % 3 > 0 )) && cursor=$((cursor - 1)) ;;
    $'\e[C' | d | l) (( cursor % 3 < 2 )) && cursor=$((cursor + 1)) ;;
    " " | "") make_move "$cursor" ;;
    1) cursor=0; make_move 0 ;;
    2) cursor=1; make_move 1 ;;
    3) cursor=2; make_move 2 ;;
    4) cursor=3; make_move 3 ;;
    5) cursor=4; make_move 4 ;;
    6) cursor=5; make_move 5 ;;
    7) cursor=6; make_move 6 ;;
    8) cursor=7; make_move 7 ;;
    9) cursor=8; make_move 8 ;;
    m | M)
      if [[ "$mode" == "cpu" ]]; then mode="2p"; else mode="cpu"; fi
      reset_board
      ;;
    r | R) reset_board ;;
    q | Q) exit 0 ;;
  esac
done
