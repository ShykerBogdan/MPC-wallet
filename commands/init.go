package commands

import (
	"fmt"

	"github.com/johnthethird/thresher/config"

	"github.com/spf13/cobra"
)

func initCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [blockchain network project nick address]",
		Short: "Initialize a new project config (default filename is ./[project]-[nick].json)",
		Long:  `Initialize a new config for a project. 

blockchain: Only 'avalanche' is supported currently
network:    'mainnet' or 'fuji'
project:    The name of your project, e.g. 'DAOTreasury'
nick:       Your nickname in the chat, e.g. 'PrezCamacho'
address:    Your Avalanche X chain address, e.g. X-fuji1xv3653....
`,
		Args: cobra.ExactArgs(5),
		RunE: func(c *cobra.Command, args []string) error {
			filename, _ := c.Flags().GetString("config")
			if filename == "" {
				filename = fmt.Sprintf("%s-%s.json", args[2], args[3])
			}
			return initProjectConfig(filename, args[0], args[1], args[2], args[3], args[4])
		},
	}

	return cmd
}

func initProjectConfig(filename string, blockchain string, network string, project string, nick string, address string) error {
	cfg, err := config.New(blockchain, network, project, nick, address)
	if err != nil {
		return err
	}
	
	err = cfg.Save(filename)
	if err != nil {
		return err
	}

	fmt.Printf("New project created with config file: %s \n", cfg.CfgFile())
	return nil
}

// func getPassPhrase(prompt string, confirmation bool) string {
// 	fmt.Print(prompt)
// 	var password string
// 	fmt.Scanln(&password)
// 	if confirmation {
// 		fmt.Print("Repeat password: ")
// 		var confirmationPassword string
// 		fmt.Scanln(&confirmationPassword)
// 		if password != confirmationPassword {
// 			utils.Fatalf("Password should match")
// 		}
// 	}
// 	return password
// }
