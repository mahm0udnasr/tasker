// Command tasker is a keyboard-driven task manager for the terminal.
package main

import (
	"os"

	"github.com/mahm0udnasr/tasker/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
