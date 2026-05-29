# prq demo — uses mock binary, no real GitHub data
Output screenshots/assets/prq.gif

Set Shell "bash"
Set FontSize 20
Set Width 2400
Set Height 1360
Set Theme "nord"
Set PlaybackSpeed 0.75

Hide
Type "bin/prq-mock"
Enter
Sleep 1500ms
Show

# Take the static screenshot on first frame (Review Queue tab)
Screenshot screenshots/assets/prq.png

# Scroll down through the review queue
Sleep 500ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 500ms

# Switch to My PRs tab
Tab
Sleep 800ms

# Scroll through My PRs
Down
Sleep 400ms
Down
Sleep 600ms

# Toggle full help
Type "?"
Sleep 1s

# Dismiss help
Escape
Sleep 500ms
