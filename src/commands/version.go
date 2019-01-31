package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	command := &Command{
		Use:   "version",
		Short: "Print version, build revision and other relevant informations",
		Long:  `Print version, build revision and other relevant informations`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
		},
	}

	RegisterCommand(command)
}
