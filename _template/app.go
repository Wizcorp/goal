package main

import (
	"os"

	_ "github.com/Wizcorp/goal/_template/src/services"
	_ "github.com/Wizcorp/goal/src/services"

	goal "github.com/Wizcorp/goal/src/commands"
)

func main() {
	if err := goal.Run(); err != nil {
		os.Exit(1)
	}
}
