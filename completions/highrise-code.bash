_highrise_code() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    case "$prev" in
        -m|--mode)
            COMPREPLY=($(compgen -W "iso 3d 2d" -- "$cur"))
            return
            ;;
        -c|--color-by)
            COMPREPLY=($(compgen -W "language depth size" -- "$cur"))
            return
            ;;
        -H|--height-by)
            COMPREPLY=($(compgen -W "lines code size flat" -- "$cur"))
            return
            ;;
        --format)
            COMPREPLY=($(compgen -W "text json" -- "$cur"))
            return
            ;;
        --out)
            COMPREPLY=($(compgen -f -- "$cur"))
            return
            ;;
        -d|--max-depth|--max-files|--max-file-size|--workers|--port|--idle-timeout|-i|--ignore)
            return
            ;;
    esac
    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "-m --mode -c --color-by -H --height-by -d --max-depth -i --ignore --include-hidden --no-gitignore --max-files --max-file-size --workers --port --no-open --idle-timeout --out --stats --format --strict -v --verbose -q --quiet -h --help -V --version" -- "$cur"))
        return
    fi
    COMPREPLY=($(compgen -d -- "$cur"))
}
complete -o filenames -F _highrise_code highrise-code
