# fish completion for hrs

set -l commands serve log ls tui migrate docs rm edit export categories version

# Disable file completions by default
complete -c hrs -f

# Top-level commands
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a serve -d "Start the hrs HTTP server"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a log -d "Log a new time entry"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a ls -d "List time entries"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a tui -d "Open the terminal UI"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a migrate -d "Run database migrations"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a docs -d "Open documentation"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a rm -d "Remove a time entry"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a edit -d "Edit an existing time entry"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a export -d "Export time entries"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a categories -d "List available categories"
complete -c hrs -n "not __fish_seen_subcommand_from $commands" -a version -d "Show version information"

# log flags
complete -c hrs -n "__fish_seen_subcommand_from log" -s c -d "Category" -xa "(hrs categories 2>/dev/null)"
complete -c hrs -n "__fish_seen_subcommand_from log" -s t -d "Title"
complete -c hrs -n "__fish_seen_subcommand_from log" -s b -d "Bullet points"
complete -c hrs -n "__fish_seen_subcommand_from log" -s e -d "Estimated hours"
complete -c hrs -n "__fish_seen_subcommand_from log" -s d -d "Date"
complete -c hrs -n "__fish_seen_subcommand_from log" -s T -d "Time"

# ls flags
complete -c hrs -n "__fish_seen_subcommand_from ls" -l format -d "Output format" -xa "table json csv"
complete -c hrs -n "__fish_seen_subcommand_from ls" -l from -d "Start date"
complete -c hrs -n "__fish_seen_subcommand_from ls" -l to -d "End date"
complete -c hrs -n "__fish_seen_subcommand_from ls" -l category -d "Filter by category" -xa "(hrs categories 2>/dev/null)"

# edit flags
complete -c hrs -n "__fish_seen_subcommand_from edit" -s c -d "Category" -xa "(hrs categories 2>/dev/null)"
complete -c hrs -n "__fish_seen_subcommand_from edit" -s t -d "Title"
complete -c hrs -n "__fish_seen_subcommand_from edit" -s b -d "Bullet points"
complete -c hrs -n "__fish_seen_subcommand_from edit" -s e -d "Estimated hours"
complete -c hrs -n "__fish_seen_subcommand_from edit" -s d -d "Date"
complete -c hrs -n "__fish_seen_subcommand_from edit" -s T -d "Time"
