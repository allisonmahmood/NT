nt() {
  emulate -L zsh
  local _ntf; _ntf="$(command mktemp -u "${TMPDIR:-/tmp}/nt-cd.XXXXXX")"
  NT_CD_FILE="$_ntf" command nt "$@"
  local _rc=$?
  if [[ -f "$_ntf" ]]; then
    local _ntd; _ntd="$(command cat -- "$_ntf")"
    command rm -f -- "$_ntf"
    [[ -n "$_ntd" ]] && builtin cd -- "$_ntd"
  fi
  return $_rc
}
