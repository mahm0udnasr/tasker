# Records docs/demo.gif with VHS (https://github.com/charmbracelet/vhs):
#   make demo
Output docs/demo.gif
Set Shell bash
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 16
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Env TASKER_DB "/tmp/tasker-demo.db"
Env XDG_CONFIG_HOME "/tmp/tasker-demo-config"

Hide
Type "rm -rf /tmp/tasker-demo.db* /tmp/tasker-demo-config && clear"
Enter
Show

# Launch; the welcome screen offers example tasks.
Type "tasker"
Enter
Sleep 1.5s
Type "s"
Sleep 1.5s

# Complete a task, then undo.
Type "jj"
Sleep 600ms
Type "x"
Sleep 1s
Type "u"
Sleep 1s

# Add a task with a natural-language due date.
Type "a"
Sleep 500ms
Type "Write the report"
Tab
Tab
Type "friday 4pm"
Sleep 800ms
Ctrl+S
Sleep 1.2s

# Search, jump, expand subtasks.
Type "/"
Sleep 400ms
Type "plan"
Sleep 800ms
Enter
Sleep 1s
Enter
Sleep 1s

# Pomodoro and focus mode.
Type "P"
Sleep 1.5s
Type "z"
Sleep 2s
Escape
Sleep 500ms

# Stats, then preview themes live.
Type "S"
Sleep 2s
Escape
Type "T"
Sleep 500ms
Down
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms
Enter
Sleep 1.2s
Type "q"
