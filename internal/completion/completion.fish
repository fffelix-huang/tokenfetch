# fish completion for tokenfetch
# Setup: tokenfetch --fish | source in ~/.config/fish/config.fish

complete -c tokenfetch -f

set -l no_range 'not __fish_seen_argument -l all -l month -l week -l today'
complete -c tokenfetch -n $no_range -l all -d 'Everything recorded (default)'
complete -c tokenfetch -n $no_range -l month -d 'Last 30 days'
complete -c tokenfetch -n $no_range -l week -d 'Last 7 days'
complete -c tokenfetch -n $no_range -l today -d 'Since local midnight'

complete -c tokenfetch -l json -d 'Print the report as JSON'
complete -c tokenfetch -l rebuild -d 'Re-read all logs'
complete -c tokenfetch -l color -x -a 'auto always never' -d 'When to use colors'
complete -c tokenfetch -s v -l version -d 'Print version'
complete -c tokenfetch -s h -l help -d 'Show help'

complete -c tokenfetch -l bash -d 'Print bash completion script'
complete -c tokenfetch -l zsh -d 'Print zsh completion script'
complete -c tokenfetch -l fish -d 'Print fish completion script'
