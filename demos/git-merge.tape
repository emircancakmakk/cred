Output assets/git-merge.gif
Require cred
Require python3
Require git
Set Shell bash
Set FontFamily "Menlo"
Set FontSize 20
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Framerate 12
Set TypingSpeed 35ms
Set CursorBlink false
Set WaitTimeout 60s

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
Sleep 3s
Type "clear"
Enter
Type "python3 demos/git_demo.py conflict"
Enter
Wait+Screen /Conflict resolution verified/
Sleep 4s
