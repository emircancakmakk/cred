Output assets/quick-start.gif
Require cred
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

Type "cred init"
Enter
Sleep 1s
Enter
Wait+Line /Profile name/
Enter
Wait+Line /Master password/
Type@1ms "demo-only-password"
Enter
Wait+Line /Repeat master password/
Type@1ms "demo-only-password"
Enter
Wait+Line /Use this vault by default/
Type "y"
Enter
Sleep 500ms
Type "cred add my-project vercel"
Enter
Wait+Line /Master password/
Type@1ms "demo-only-password"
Enter
Wait+Line /Field name/
Type "password"
Enter
Wait+Line /Value:/
Type@1ms "example-demo-token"
Enter
Wait+Line /Secret?/
Type "y"
Enter
Wait+Line /Field name/
Enter
Wait+Screen /added 1 field/
Sleep 1s
Type "cred"
Enter
Wait+Line /Master password/
Type@1ms "demo-only-password"
Enter
Sleep 2s
Enter
Sleep 1500ms
Enter
Sleep 3s
Type "q"
Sleep 500ms
