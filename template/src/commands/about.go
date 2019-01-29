package commands

import (
	"fmt"

	. "github.com/Wizcorp/goal/src/commands"
	"github.com/spf13/cobra"
)

func init() {
	command := &Command{
		Use:   "about",
		Short: "General information about this project",
		Long:  `General information about this project`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
		},
	}

	AddCommand(command)
}
