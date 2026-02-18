# bash completion for unfuck-ai-comments (generated via go-flags)
_unfuck_ai_comments() {
    local args=("${COMP_WORDS[@]:1:$COMP_CWORD}")
    mapfile -t COMPREPLY < <(GO_FLAGS_COMPLETION=1 "${COMP_WORDS[0]}" "${args[@]}" 2>/dev/null)
    return 0
}
complete -o default -F _unfuck_ai_comments unfuck-ai-comments
