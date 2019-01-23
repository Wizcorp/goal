package main

import (
	"os"

	goal "github.com/Wizcorp/goal/src/commands"
	_ "github.com/Wizcorp/goal/src/services"
)

func main() {
	if err := goal.Run(); err != nil {
		os.Exit(1)
	}
}
