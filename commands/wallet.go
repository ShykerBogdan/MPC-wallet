package commands

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shykerbogdan/mpc-wallet/config"
	"github.com/shykerbogdan/mpc-wallet/network/chat"
	"github.com/spf13/cobra"
)

func walletCommand() *cobra.Command {
	var bootstrapaddrs []string
	var listenaddrs []string

	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Start a wallet session",
		Long:  ``,
		Run: func(c *cobra.Command, args []string) {
			appConfig.MustExist()

			logFileName, _ := c.Flags().GetString("log")
			setLogOutput(logFileName)
			log.Println(appConfig.String())

			fmt.Println(appConfig.String())
			fmt.Printf("STT wallet chat session started, logging to %s \n", logFileName)
			fmt.Println("(This could take a while to connect to libp2p network)")

			runChatCmd(appConfig, bootstrapaddrs, listenaddrs)
		},
	}

	cmd.Flags().StringSliceVar(&bootstrapaddrs, "bootstrap", []string{}, "bootstrap addrs")
	cmd.Flags().StringSliceVar(&listenaddrs, "listen", []string{}, "listen addrs")
	// TODO Where should we put the config and log files?
	cmd.Flags().StringP("config", "c", "thresher.json", "config file which **contains secrets**")
	cmd.Flags().StringP("log", "l", "thresher.log", "logfile")

	return cmd
}

func runChatCmd(cfg *config.AppConfig, bootstrapaddrs []string, listenaddrs []string) {
	nick := cfg.Me.Nick

	p2phost := chat.NewP2P(cfg.Me, cfg.Project, bootstrapaddrs, listenaddrs)
	log.Printf("Connecting to libp2p network with peerID %s listening on %v", p2phost.Host.ID().Pretty(), p2phost.Host.Addrs())

	p2phost.AnnounceConnect()
	// p2phost.AdvertiseConnect()

	chatapp, err := chat.JoinChatRoom(p2phost, cfg)
	if err != nil {
		log.Fatalf("Error joining chatroom %v", err)
	}

	net := chat.NewNetwork(chatapp)
	log.Printf("Joined chatroom with nick %s, waiting for network to start...", nick)
	// Wait for network setup to complete
	time.Sleep(time.Second * 1)

	ui := chat.NewUI(chatapp, net)
	if err := ui.Run(); err != nil {
		log.Fatalf("Error starting ui %v", err)
	}
}

func setLogOutput(filename string) {
	// Just use default stderr if no name specified
	if filename == "" {
		return
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	log.SetOutput(file)
}
