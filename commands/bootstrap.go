package commands

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	libp2plog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	p2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
)

func bootstrapCommand() *cobra.Command {
	var port int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Start a private libp2p bootstrap server",
		Long: `
Thresher uses libp2p to facilitate peer-to-peer communication between participants. 
A "bootstrap server" is a node that all peers initially connect to, so that they can all find each other
and that can help coordinate peer-to-peer connections across various obstacles like NATs, etc.
If you run thresher without specifying a bootstrap server, thresher will use the public
servers run by the libp2p project (which can be slow). This command will run a private bootstrap server,
which should be run on a machine with direct access to the Internet, either with 
a directly accessible IP or with the appropriate port-forwarding rules in place such that
all nodes can make an incoming connection to the bootstrap server.

No matter what bootstrap servers are used, *all* libp2p communications between peers are *always* encrypted.
		`,
		Run: func(c *cobra.Command, args []string) {
			if verbose {
				libp2plog.SetAllLoggers(libp2plog.LevelDebug)
			}
			peerkey := getOrCreatePeerKey()
			startServer(port, peerkey)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 4001, "TCP port the server should listen on")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable (very) verbose libp2p debug logging")

	return cmd
}

// If we dont have a libp2p peer private key, generate one and store it, so that our peer address
// can remain the same and not change on each run, mainly for convenience
func getOrCreatePeerKey() crypto.PrivKey {
	var prvkey crypto.PrivKey

	p := appDirs.UserConfig()
	_ = os.MkdirAll(p, 0700)
	keyfilename := filepath.Join(p, "peerprivkey.dat")
	data, err := os.ReadFile(keyfilename)
	if err == nil {
		fmt.Printf("Loading libp2p peer private key file %s\n", keyfilename)
		prvkey, err = crypto.UnmarshalPrivateKey(data)
		if err != nil {panic(err)}
	// peerkey.dat doesnt exist, so lets make one and store it
	} else {
		fmt.Printf("Libp2p peer private key file %s not found, generating new key", keyfilename)
		f, err := os.OpenFile(keyfilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {panic(err)}
		defer f.Close()

		prvkey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {panic(err)}

		prvkeybytes, err := crypto.MarshalPrivateKey(prvkey)
		if err != nil {panic(err)}

		_, err = f.Write(prvkeybytes)
		if err != nil {panic(err)}

	}

	return prvkey
}

func startServer(port int, prvkey crypto.PrivKey) {
	ctx := context.Background()

	// Ensure our public IP is advertised even if we are running on Docker or whatever
	publicIP, err := getPublicIP()
	if err != nil {panic(err)}
	tcpBindAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
	if err != nil {panic(err)}
	tcpAdvertiseAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", publicIP, port))
	if err != nil {panic(err)}
	advertiseAddrs := []ma.Multiaddr{tcpAdvertiseAddr}

	// The inner function will set kaddht var when it runs during libp2p.New
	var kaddht *dht.IpfsDHT
	routing := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		var err error
		kaddht, err = dht.New(
			ctx, 
			h, 
			dht.Mode(dht.ModeServer), 
			dht.BootstrapPeers(),
		)
		return kaddht, err
	})

	// Dont think we need a connection manager for Thresher, since its for small private groups
	// (require) connmgr "github.com/libp2p/go-libp2p-connmgr"
	// connManager := connmgr.NewConnManager(config.PeerCountLow, config.PeerCountHigh, peerGraceDuration)
	// option => libp2p.ConnectionManager(connManager)

	host, err := libp2p.New(
		routing,
		libp2p.Identity(prvkey),
		libp2p.ListenAddrs(tcpBindAddr),
		libp2p.AddrsFactory(newAddrsFactory(advertiseAddrs)),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableRelayService(),
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
	)
	if err != nil {panic(err)}
	
	host.Network().Notify(&notifee{})

	fmt.Println("")
	fmt.Printf("[*] Libp2p Bootstrap Server Is Listening On: \n\n")
	for _, a := range host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", a.String(), host.ID().Pretty())
	}
	fmt.Println("")

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\nReceived signal, shutting down...")

	// shut the node down
	if err := host.Close(); err != nil {
		panic(err)
	}	
	
}

func newAddrsFactory(advertiseAddrs []ma.Multiaddr) func([]ma.Multiaddr) []ma.Multiaddr {
	return func(addrs []ma.Multiaddr) []ma.Multiaddr {
		// Note that we append the advertiseAddrs here just in case we are not
		// actually reachable at our public IP address (and are reachable at one of
		// the other addresses).
		return append(addrs, advertiseAddrs...)
	}
}

// notifee receives notifications for network-related events.
type notifee struct{}

var _ p2pnet.Notifiee = &notifee{}

// Listen is called when network starts listening on an addr
func (n *notifee) Listen(p2pnet.Network, ma.Multiaddr) {}

// ListenClose is called when network stops listening on an addr
func (n *notifee) ListenClose(p2pnet.Network, ma.Multiaddr) {}

// Connected is called when a connection opened
func (n *notifee) Connected(network p2pnet.Network, conn p2pnet.Conn) {
	fmt.Printf("Connected to %s/p2p/%s\n", conn.RemoteMultiaddr(), conn.RemotePeer())
}

// Disconnected is called when a connection closed
func (n *notifee) Disconnected(network p2pnet.Network, conn p2pnet.Conn) {
	fmt.Printf("Disconnected to %s/p2p/%s\n", conn.RemoteMultiaddr(), conn.RemotePeer())
}

// OpenedStream is called when a stream opened
func (n *notifee) OpenedStream(network p2pnet.Network, stream p2pnet.Stream) {}

// ClosedStream is called when a stream closed
func (n *notifee) ClosedStream(network p2pnet.Network, stream p2pnet.Stream) {}


// From https://github.com/0xProject/0x-mesh
func fetchPublicIPFromExternalSource(source string) (string, error) {
	res, err := http.Get(source)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	ipBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(ipBytes), "\n"), nil
}

func getPublicIP() (string, error) {
	sources := []string{"https://wtfismyip.com/text", "https://whatismyip.api.0x.org", "https://ifconfig.me/ip"}
	// sources = append(additionalSources, sources...)
	for _, source := range sources {
		ip, err := fetchPublicIPFromExternalSource(source)
		if err != nil {
			fmt.Printf("failed to get public ip from source: %s\n", source)
			continue
		}

		return ip, nil
	}

	return "", errors.New("failed to get public ip from all provided external sources")
}
