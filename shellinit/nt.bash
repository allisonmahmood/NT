nt() {
  local _ntf; _ntf="$(command mktemp "${TMPDIR:-/tmp}/nt-cd.XXXXXX")" || return 1
  NT_CD_FILE="$_ntf" command nt "$@"
  local _rc=$?
  if [ -f "$_ntf" ]; then
    local _ntd; _ntd="$(command cat -- "$_ntf")"
    command rm -f -- "$_ntf"
    [ -n "$_ntd" ] && cd -- "$_ntd"
  fi
  return $_rc
}
