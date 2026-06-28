function nt
    set -l _ntf (command mktemp); or return 1
    NT_CD_FILE=$_ntf command nt $argv
    set -l _rc $status
    if test -f $_ntf
        set -l _ntd (command cat -- $_ntf)
        command rm -f -- $_ntf
        test -n "$_ntd"; and builtin cd -- "$_ntd"
    end
    return $_rc
end
