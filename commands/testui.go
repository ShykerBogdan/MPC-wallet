package commands

import (
	"github.com/johnthethird/thresher/network/chat"
	"github.com/spf13/cobra"
)

func testUICommand() *cobra.Command {
	cmdchan := make(chan chat.UICommand, 100)
	msgchan := make(chan string, 100)

	cmd := &cobra.Command{
		Use:   "testui",
		Short: "All software has versions.",
		Run: func(c *cobra.Command, args []string) {
			appConfig.MustExist()
			ui := chat.NewTerminalApp("blockchain", "room", "nick", cmdchan, msgchan)
			_ = ui.TerminalApp.Run()
		},
	}

	return cmd
}
