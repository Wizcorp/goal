package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	binaryName := filepath.Base(os.Args[0])
	long := fmt.Sprintf("Usage: cd $(%s develop)", binaryName)
	command := &Command{
		Use:   "develop",
		Short: "Outputs where the binary's project is locally located",
		Long:  long,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getProjectDir())
		},
	}

	AddCommand(command)
}
