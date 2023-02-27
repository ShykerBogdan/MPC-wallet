package commands

import (
	"fmt"

	"github.com/johnthethird/thresher/version"

	"github.com/spf13/cobra"
)

func debugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Print debug info including all UTXOs for public keys in all wallets.",
		Run: func(c *cobra.Command, args []string) {
			fmt.Println("Build Date:", version.BuildDate)
			fmt.Println("Git Commit:", version.GitCommit)
			fmt.Println("Version:", version.Version)
			fmt.Println("Go Version:", version.GoVersion)
			fmt.Println("OS / Arch:", version.OsArch)

			appConfig.MustExist()

			for _, w := range appConfig.Wallets {
				_ = w.FetchUTXOs()
				fmt.Println("")
				fmt.Printf("  Wallet %s: Addr: %s \n", w.Name, w.GetFormattedAddress())
				for aid, amt := range w.GetBalances() {
					asset := w.GetAsset(aid)
					bal := w.FormatAmount(asset, amt)
					fmt.Printf("    %s[%s]: %v \n", asset.Name, asset.Symbol, bal)
				}
				fmt.Printf("  UTXOs:\n%s\n", w.DumpUTXOs())
			}
		},
	}

	return cmd
}
