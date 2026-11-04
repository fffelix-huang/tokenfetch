# bash completion for tokenfetch
# Setup: eval "$(tokenfetch --bash)" in ~/.bashrc

# _tokenfetch_value FLAG VALUES: complete the value of --FLAG=..., return 1
# when the cursor isn't on one. Readline completes only the text after "=",
# so reply with bare values. bash 4+ splits "--color=al" into "--color" "="
# "al"; bash 3.2 keeps one word.
_tokenfetch_value() {
  local flag=$1 values=$2 value
  local cur=${COMP_WORDS[COMP_CWORD]}
  local prev=${COMP_WORDS[COMP_CWORD-1]}
  if [[ $cur == $flag=* ]]; then
    value=${cur#$flag=}
  elif [[ $cur == = && $prev == $flag ]]; then
    value=
  elif [[ $prev == $flag || ( $prev == = && ${COMP_WORDS[COMP_CWORD-2]} == $flag ) ]]; then
    value=$cur
  else
    return 1
  fi
  COMPREPLY=($(compgen -W "$values" -- "$value"))
}

_tokenfetch() {
  local cur=${COMP_WORDS[COMP_CWORD]}

  case ${COMP_WORDS[1]} in
    service)
      _tokenfetch_value --interval "hourly daily weekly" && return
      if [[ $COMP_CWORD == 2 ]]; then
        COMPREPLY=($(compgen -W "install uninstall status" -- "$cur"))
      elif [[ ${COMP_WORDS[2]} == install ]]; then
        COMPREPLY=($(compgen -W "--interval" -- "$cur"))
      fi
      return
      ;;
    ingest)
      [[ $COMP_CWORD == 1 ]] || return
      ;;
  esac

  _tokenfetch_value --color "auto always never" && return

  # Ranges are mutually exclusive: offer them only until one is given.
  local ranges="--all --month --week --today" word
  for word in "${COMP_WORDS[@]:1:COMP_CWORD-1}"; do
    case $word in --all|--month|--week|--today) ranges= ;; esac
  done
  local words="$ranges --json --rebuild --color -v --version -h --help --bash --zsh --fish"
  # Commands only as the first word.
  [[ $COMP_CWORD == 1 ]] && words="ingest service $words"
  COMPREPLY=($(compgen -W "$words" -- "$cur"))
}

complete -F _tokenfetch tokenfetch
