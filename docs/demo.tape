# The README's demo GIF. Re-record it from the repository root with
#
#   vhs docs/demo.tape
#
# Needs VHS (https://github.com/charmbracelet/vhs) and Go. It builds rolecall, copies the
# showcase Workspace (examples/workspace) to a temporary folder so the recorded move leaves
# the checked-in copy untouched, and runs against an empty config.toml.

Output docs/demo.gif

Set Shell bash
Set FontSize 15
Set Width 1280
Set Height 760
Set Padding 16
Set Framerate 24
Set TypingSpeed 0ms

# The showcase Owner lives in Melbourne; pin the theme so the recording does not depend on
# the terminal VHS reports.
Env TZ "Australia/Melbourne"
Env ROLECALL_THEME "dark"

Hide
Type "unset NO_COLOR ROLECALL_PLAIN ROLECALL_ASCII ROLECALL_PLUGIN"
Enter
Type "export W=$(mktemp -d) XDG_CONFIG_HOME=$(mktemp -d)"
Enter
Type "cp -R examples/workspace/. $W && go build -o $W.bin ./cmd/rolecall && clear"
Enter
Sleep 10s
Type "$W.bin --dir $W"
Enter
Sleep 2s
Show

# Leads: what came in, with its Fit.
Sleep 2s
Down
Sleep 1.5s

# Drafts, then the Pipeline: a follow-up due, an interview booked, an offer.
Type "2"
Sleep 1.5s
Type "3"
Sleep 1.5s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1.5s

# Stats and the Profile.
Type "4"
Sleep 2.5s
Type "5"
Sleep 2s

# Open a Draft and read its Resume.
Type "2"
Sleep 1s
Enter
Sleep 1.5s
Right
Sleep 500ms
Right
Sleep 1.5s
Down 6
Sleep 2s

# It has been sent: move it to Applied, then follow it to the Pipeline.
Type "m"
Sleep 2s
Type "a"
Sleep 2s
Type "i"
Sleep 3s

Hide
Type "q"
Sleep 500ms
Type "rm -rf $W $W.bin $XDG_CONFIG_HOME"
Enter
