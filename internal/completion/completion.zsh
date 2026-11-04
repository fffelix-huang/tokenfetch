#compdef tokenfetch
# zsh completion for tokenfetch
# Setup: source <(tokenfetch --zsh) in ~/.zshrc, or install as _tokenfetch in $fpath

_tokenfetch() {
  case $words[2] in
    service)
      shift 2 words
      (( CURRENT -= 2 ))
      if (( CURRENT == 1 )); then
        local -a cmds=(
          'install:install the periodic job and ingest once now'
          'uninstall:remove the periodic job, keep recorded usage'
          'status:show whether the periodic job is installed and healthy'
        )
        _describe command cmds
      elif [[ $words[1] == install ]]; then
        _arguments '--interval=[how often to run]:every:(hourly daily weekly)'
      fi
      return
      ;;
    ingest)
      (( CURRENT == 2 )) || return
      ;;
  esac

  # Commands only as the first word.
  if (( CURRENT == 2 )) && [[ $PREFIX != -* ]]; then
    local -a cmds=(
      'ingest:record new usage without printing a report'
      'service:manage the periodic ingest job'
    )
    _describe command cmds
    return
  fi

  local ranges='(--all --month --week --today)'
  _arguments -s \
    "${ranges}--all[everything recorded, activity over the past year (default)]" \
    "${ranges}--month[last 30 days]" \
    "${ranges}--week[last 7 days]" \
    "${ranges}--today[since local midnight]" \
    '--json[print the report as JSON]' \
    '--rebuild[re-read all logs]' \
    '--color=[when to use colors]:when:(auto always never)' \
    '(- *)'{-v,--version}'[print version]' \
    '(- *)'{-h,--help}'[show help]' \
    '(- *)--bash[print bash completion script]' \
    '(- *)--zsh[print zsh completion script]' \
    '(- *)--fish[print fish completion script]'
}

# Autoloaded from $fpath: run now. Sourced: register.
if [[ $funcstack[1] == _tokenfetch ]]; then
  _tokenfetch "$@"
else
  compdef _tokenfetch tokenfetch
fi
