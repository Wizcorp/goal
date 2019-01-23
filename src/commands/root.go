package commands

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Wizcorp/goal/src/systems"
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"

	. "github.com/Wizcorp/goal/src/api"
	. "github.com/Wizcorp/goal/src/systems"
)

// Command struct
type Command = cobra.Command

// Singleton root command
var command = &cobra.Command{
	Use:   "goal",
	Short: "Goal NextGen Game Server",
	Long:  `GoalNG is a game server framework for real-time games`,
	Run: func(cmd *cobra.Command, args []string) {
		config := LoadConfig()
		server := systems.NewServer(config)
		server.RegisterDefaultSystems()
		err := server.Start()

		if err != nil {
			log.Fatalf("Failed to start the server:\n\n %s", errors.Wrap(err, 0).ErrorStack())
			os.Exit(1)
		}

		logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
		interrupt := make(chan os.Signal, 1)
		exit := make(chan int)

		signal.Notify(
			interrupt,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
		)

		shutdown := func(s os.Signal) {
			os.Stdout.WriteString("\r")
			logger.Infof("Received signal %v, shutting down", s)
			err := server.Stop()

			if err != nil {
				exit <- 1
			} else {
				exit <- 0
			}
		}

		handleSignal := func(interrupt chan os.Signal, config *GoalConfig) {
			for {
				s := <-interrupt
				switch s {
				case syscall.SIGHUP:
					shutdown(s)
				case syscall.SIGINT:
					shutdown(s)
				case syscall.SIGTERM:
					shutdown(s)
				case syscall.SIGQUIT:
					shutdown(s)
				}
			}
		}

		go handleSignal(interrupt, config)

		code := <-exit
		os.Exit(code)
	},
}

// AddCommand allows developers to add their own custom commands
// when needed
func AddCommand(cmd *cobra.Command) {
	command.AddCommand(cmd)
}

// Run is the entry point for the command line interface
func Run() error {
	return command.Execute()
}
