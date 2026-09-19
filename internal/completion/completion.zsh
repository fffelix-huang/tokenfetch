#compdef tokenfetch
# zsh completion for tokenfetch
# Setup: source <(tokenfetch --zsh) in ~/.zshrc, or install as _tokenfetch in $fpath

_tokenfetch() {
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
