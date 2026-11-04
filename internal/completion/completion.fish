# fish completion for tokenfetch
# Setup: tokenfetch --fish | source in ~/.config/fish/config.fish

complete -c tokenfetch -f

# Commands only as the first word; report flags only outside them.
set -l first 'test (count (commandline -opc)) -eq 1'
set -l top 'not __fish_seen_subcommand_from ingest service'
complete -c tokenfetch -n $first -a ingest -d 'Record new usage without printing a report'
complete -c tokenfetch -n $first -a service -d 'Manage the periodic ingest job'

set -l service_cmd '__fish_seen_subcommand_from service; and not __fish_seen_subcommand_from install uninstall status'
complete -c tokenfetch -n $service_cmd -a install -d 'Install the periodic job and ingest once now'
complete -c tokenfetch -n $service_cmd -a uninstall -d 'Remove the periodic job, keep recorded usage'
complete -c tokenfetch -n $service_cmd -a status -d 'Show whether the periodic job is installed'
complete -c tokenfetch -n '__fish_seen_subcommand_from service; and __fish_seen_subcommand_from install' -l interval -x -a 'hourly daily weekly' -d 'How often to run'

set -l no_range "$top; and not __fish_seen_argument -l all -l month -l week -l today"
complete -c tokenfetch -n $no_range -l all -d 'Everything recorded (default)'
complete -c tokenfetch -n $no_range -l month -d 'Last 30 days'
complete -c tokenfetch -n $no_range -l week -d 'Last 7 days'
complete -c tokenfetch -n $no_range -l today -d 'Since local midnight'

complete -c tokenfetch -n $top -l json -d 'Print the report as JSON'
complete -c tokenfetch -n $top -l rebuild -d 'Re-read all logs'
complete -c tokenfetch -n $top -l color -x -a 'auto always never' -d 'When to use colors'
complete -c tokenfetch -n $top -s v -l version -d 'Print version'
complete -c tokenfetch -n $top -s h -l help -d 'Show help'

complete -c tokenfetch -n $top -l bash -d 'Print bash completion script'
complete -c tokenfetch -n $top -l zsh -d 'Print zsh completion script'
complete -c tokenfetch -n $top -l fish -d 'Print fish completion script'
