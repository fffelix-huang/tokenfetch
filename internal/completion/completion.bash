# bash completion for tokenfetch
# Setup: eval "$(tokenfetch --bash)" in ~/.bashrc

_tokenfetch() {
  local cur=${COMP_WORDS[COMP_CWORD]}
  local prev=${COMP_WORDS[COMP_CWORD-1]}

  # Readline completes only the text after "=", so reply with bare values.
  # bash 4+ splits "--color=al" into "--color" "=" "al"; bash 3.2 keeps one word.
  local value complete_value=1
  if [[ $cur == --color=* ]]; then
    value=${cur#--color=}
  elif [[ $cur == = && $prev == --color ]]; then
    value=
  elif [[ $prev == --color || ( $prev == = && ${COMP_WORDS[COMP_CWORD-2]} == --color ) ]]; then
    value=$cur
  else
    complete_value=
  fi
  if [[ -n $complete_value ]]; then
    COMPREPLY=($(compgen -W "auto always never" -- "$value"))
    return
  fi

  # Ranges are mutually exclusive: offer them only until one is given.
  local ranges="--all --month --week --today" word
  for word in "${COMP_WORDS[@]:1:COMP_CWORD-1}"; do
    case $word in --all|--month|--week|--today) ranges= ;; esac
  done
  local flags="$ranges --json --rebuild --color -v --version -h --help --bash --zsh --fish"
  COMPREPLY=($(compgen -W "$flags" -- "$cur"))
}

complete -F _tokenfetch tokenfetch
