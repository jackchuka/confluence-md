# Regenerate with: vhs assets/demo.tape (from the repository root)

Output assets/demo.gif

Set Shell "bash"
Set FontSize 22
Set Width 1440
Set Height 940
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type `export D=/tmp/confluence-md-demo && rm -rf "$D" && mkdir -p "$D" && go build -o "$D/confluence-md" ./cmd/confluence-md && cp assets/demo/getting-started.html "$D" && cd "$D" && export PATH="$D:$PATH" PS1='\n\[\e[1;35m\]$ \[\e[0m\]' && clear`
Enter
Sleep 8s
Show

Type "cat getting-started.html"
Sleep 400ms
Enter
Sleep 4s

Type "clear"
Enter
Type "confluence-md html getting-started.html -o getting-started.md"
Sleep 400ms
Enter
Sleep 1500ms
Type "cat getting-started.md"
Sleep 400ms
Enter
Sleep 5s
