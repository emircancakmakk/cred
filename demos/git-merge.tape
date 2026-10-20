Output assets/git-merge.gif
Require cred
Require python3
Require git
Set Shell bash
Set FontFamily "Menlo"
Set FontSize 20
Set Width 1000
Set Height 600
Set Theme {"name": "Cred Black", "background": "#000000", "foreground": "#e5e5e5", "black": "#000000", "red": "#f87171", "green": "#4ade80", "yellow": "#facc15", "blue": "#60a5fa", "magenta": "#c084fc", "cyan": "#22d3ee", "white": "#e5e5e5", "brightBlack": "#737373", "brightRed": "#fca5a5", "brightGreen": "#86efac", "brightYellow": "#fde047", "brightBlue": "#93c5fd", "brightMagenta": "#d8b4fe", "brightCyan": "#67e8f9", "brightWhite": "#ffffff", "selection": "#262626", "cursor": "#e5e5e5"}
Set Padding 20
Set Framerate 12
Set TypingSpeed 70ms
Set PlaybackSpeed 0.8
Set CursorBlink false
Set WaitTimeout 120s

Hide
Type "python3 demos/git_demo.py setup"
Enter
Wait+Screen /DEMO READY/
Type "clear"
Enter
Show
Type "python3 demos/git_demo.py merge"
Enter
Wait+Screen /Independent edits verified/
Sleep 5s
Type "clear"
Enter
Type "python3 demos/git_demo.py conflict"
Enter
Wait+Screen /Conflict resolution verified/
Sleep 5s
