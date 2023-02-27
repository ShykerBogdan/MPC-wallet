package commands

import (
	"fmt"

	"github.com/johnthethird/thresher/version"
	"github.com/spf13/cobra"
)

func versionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of STT wallet",
		Run: func(c *cobra.Command, args []string) {
			fmt.Println("Build Date:", version.BuildDate)
			fmt.Println("Git Commit:", version.GitCommit)
			fmt.Println("Version:", version.Version)
			fmt.Println("Go Version:", version.GoVersion)
			fmt.Println("OS / Arch:", version.OsArch)
		},
	}

	return cmd
}
