#compdef hrs

# zsh completion for hrs

_hrs() {
    local -a commands
    commands=(
        'serve:Start the hrs HTTP server'
        'log:Log a new time entry'
        'ls:List time entries'
        'tui:Open the terminal UI'
        'migrate:Run database migrations'
        'docs:Open documentation'
        'rm:Remove a time entry'
        'edit:Edit an existing time entry'
        'export:Export time entries'
        'categories:List available categories'
        'version:Show version information'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe -t commands 'hrs command' commands
            ;;
        args)
            case $words[1] in
                log)
                    _arguments \
                        '-c[Category]:category:->categories' \
                        '-t[Title]:title:' \
                        '-b[Bullet points]:bullets:' \
                        '-e[Estimated hours]:hours:' \
                        '-d[Date]:date:' \
                        '-T[Time]:time:'
                    ;;
                ls)
                    _arguments \
                        '--format[Output format]:format:(md json)' \
                        '--from[Start date]:date:' \
                        '--to[End date]:date:' \
                        '--category[Filter by category]:category:->categories'
                    ;;
                export)
                    _arguments \
                        '--format[Output format]:format:(json csv)' \
                        '--from[Start date]:date:' \
                        '--to[End date]:date:' \
                        '--category[Filter by category]:category:->categories'
                    ;;
                rm)
                    _arguments \
                        '1:entry ID:'
                    ;;
                edit)
                    _arguments \
                        '1:entry ID:' \
                        '-c[Category]:category:->categories' \
                        '-t[Title]:title:' \
                        '-b[Bullet points]:bullets:' \
                        '-e[Estimated hours]:hours:' \
                        '-d[Date]:date:' \
                        '-T[Time]:time:'
                    ;;
            esac

            case $state in
                categories)
                    local -a cats
                    cats=(${(f)"$(hrs categories 2>/dev/null)"})
                    _describe -t categories 'category' cats
                    ;;
            esac
            ;;
    esac
}

_hrs "$@"
