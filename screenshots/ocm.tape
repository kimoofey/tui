# ocm demo — uses mock binary, no real SQLite data
Output screenshots/assets/ocm.gif

Set Shell "bash"
Set FontSize 20
Set Width 2400
Set Height 1360
Set Theme "nord"
Set PlaybackSpeed 0.75

Hide
Type "bin/ocm-mock --cost-period month"
Enter
Sleep 800ms
Show

# Take the static screenshot on first frame
Screenshot screenshots/assets/ocm.png

# Scroll down through sessions
Sleep 400ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 500ms

# Delete a session (dd — two-press confirmation → success)
Type "d"
Sleep 400ms
Type "d"
Sleep 800ms

# Scroll back to top
Up 10
Sleep 500ms

# Vacuum the database (vv → completes successfully)
Type "v"
Sleep 400ms
Type "v"
Sleep 1s

# Toggle full help
Type "?"
Sleep 1s

# Dismiss help
Escape
Sleep 500ms
