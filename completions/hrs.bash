#!/usr/bin/env bash
# bash completion for hrs

_hrs() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="serve log ls tui migrate docs rm edit export categories version"

    case "${COMP_WORDS[1]}" in
        log)
            case "$prev" in
                -c) COMPREPLY=($(compgen -W "$(hrs categories 2>/dev/null)" -- "$cur")); return ;;
                -d|-T) return ;;
                -t|-b) return ;;
                -e) return ;;
            esac
            COMPREPLY=($(compgen -W "-c -t -b -e -d -T" -- "$cur"))
            return
            ;;
        ls)
            case "$prev" in
                --format) COMPREPLY=($(compgen -W "md json" -- "$cur")); return ;;
                --from|--to) return ;;
                --category) COMPREPLY=($(compgen -W "$(hrs categories 2>/dev/null)" -- "$cur")); return ;;
            esac
            COMPREPLY=($(compgen -W "--format --from --to --category" -- "$cur"))
            return
            ;;
        edit)
            case "$prev" in
                -c) COMPREPLY=($(compgen -W "$(hrs categories 2>/dev/null)" -- "$cur")); return ;;
                -d|-T) return ;;
                -t|-b) return ;;
                -e) return ;;
            esac
            COMPREPLY=($(compgen -W "-c -t -b -e -d -T" -- "$cur"))
            return
            ;;
        export)
            case "$prev" in
                --format) COMPREPLY=($(compgen -W "json csv" -- "$cur")); return ;;
                --from|--to) return ;;
                --category) COMPREPLY=($(compgen -W "$(hrs categories 2>/dev/null)" -- "$cur")); return ;;
            esac
            COMPREPLY=($(compgen -W "--format --from --to --category" -- "$cur"))
            return
            ;;
        rm|docs|serve|tui|migrate|categories|version)
            return
            ;;
    esac

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    fi
}

complete -F _hrs hrs
