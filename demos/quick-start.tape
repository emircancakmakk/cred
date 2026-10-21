Output assets/quick-start.gif
Require cred
Set Shell bash
Set FontFamily "Menlo"
Set FontSize 20
Set Width 1000
Set Height 720
Set Theme {"name": "Cred Black", "background": "#000000", "foreground": "#e5e5e5", "black": "#000000", "red": "#f87171", "green": "#4ade80", "yellow": "#facc15", "blue": "#60a5fa", "magenta": "#c084fc", "cyan": "#22d3ee", "white": "#e5e5e5", "brightBlack": "#737373", "brightRed": "#fca5a5", "brightGreen": "#86efac", "brightYellow": "#fde047", "brightBlue": "#93c5fd", "brightMagenta": "#d8b4fe", "brightCyan": "#67e8f9", "brightWhite": "#ffffff", "selection": "#262626", "cursor": "#e5e5e5"}
Set Padding 20
Set Framerate 12
Set TypingSpeed 70ms
Set PlaybackSpeed 0.8
Set CursorBlink false
Set WaitTimeout 120s

Type "cred init"
Enter
Wait+Screen /Choose a vault/
Sleep 1500ms
Enter
Wait+Screen /Vault details/
Sleep 1500ms
Enter 2
Wait+Screen /Create a password/
Sleep 1s
Type@55ms "demo-only-password"
Enter
Type@55ms "demo-only-password"
Enter 2
Wait+Screen /Review vault/
Sleep 2500ms
Enter
Wait+Screen /Vault ready/
Sleep 1500ms
Down
Enter
Wait+Screen /Default vault: local/
Enter
Wait+Screen /Ready: local/
Type "cred"
Enter
Wait+Screen /Master password/
Type@55ms "demo-only-password"
Enter 2
Wait+Screen /No projects/
Sleep 1500ms

# Create the project, service and secret without leaving the browser.
Type "n"
Wait+Screen /New project/
Sleep 1s
Type "my-project"
Enter 2
Wait+Screen /created my-project/
Sleep 1500ms
Enter
Wait+Screen /No services/
Type "n"
Wait+Screen /New service/
Sleep 1s
Type "vercel"
Enter 2
Wait+Screen /created vercel/
Sleep 1500ms
Enter
Wait+Screen /No fields/
Type "n"
Wait+Screen /Choose field name/
Sleep 1s
Down
Enter
Type@55ms "example-demo-token"
Sleep 1500ms
Ctrl+r
Wait+Screen /example-demo-token/
Sleep 2500ms
Ctrl+r
Enter 3
Wait+Screen /created password/
Sleep 2500ms

# Reveal the fake value, then show the edit form. Never copy it.
Type "s"
Wait+Screen /example-demo-token/
Sleep 2500ms
Type "e"
Wait+Screen /Edit field/
Sleep 3s
Escape
Sleep 1500ms
Type "q"
Sleep 1500ms
