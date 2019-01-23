package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	. "github.com/Wizcorp/goal/src/api"
)

func init() {
	command := &Command{
		Use:   "proto-paths",
		Short: "Print version, build revision and other relevant informations",
		Long:  `Print version, build revision and other relevant informations`,
		Run: func(cmd *cobra.Command, args []string) {
			list := ""

			for _, file := range ListProtoFiles() {
				list = fmt.Sprintf("%s %s", list, file)
			}

			fmt.Print(list)
		},
	}

	AddCommand(command)
}
