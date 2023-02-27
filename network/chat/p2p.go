package chat

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/johnthethird/thresher/user"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/mr-tron/base58/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
)

// A structure that represents a P2P Host
type P2P struct {
	Ctx          context.Context
	Me           user.Me
	Host         core.Host
	KadDHT       *dht.IpfsDHT
	Discovery    *discovery.RoutingDiscovery
	PubSub       *pubsub.PubSub
	ChatroomName string
}

/*
A constructor function that generates and returns a P2P object.

Constructs a libp2p host with TLS encrypted secure transportation that works over a TCP
transport connection using a Yamux Stream Multiplexer and uses UPnP for the NAT traversal.

A Kademlia DHT is then bootstrapped on this host using the specified peers
and a Peer Discovery service is created from this Kademlia DHT. The PubSub handler is then
created on the host using the peer discovery service created prior.
*/
func NewP2P(me user.Me, chatroomname string, bootstrapaddrs []string, listenaddrs []string) *P2P {
	ctx := context.Background()

	nodehost, kaddht := setupHostAndDHT(ctx, bootstrapaddrs, listenaddrs, me)
	log.Printf("Created the P2P Host and the Kademlia DHT")

	bootstrapDHT(ctx, nodehost, kaddht, bootstrapaddrs)
	log.Printf("Bootstrapped the Kademlia DHT and Connected to Bootstrap Peers %s", strings.Join(bootstrapaddrs, ", "))

	routingdiscovery := discovery.NewRoutingDiscovery(kaddht)
	log.Printf("Created the Peer Discovery Service")

	pubsubhandler := setupPubSub(ctx, nodehost, routingdiscovery)
	log.Printf("Created the PubSub Handler")

	
	p2phost := &P2P{
		Ctx:          ctx,
		Me:           me,
		Host:         nodehost,
		KadDHT:       kaddht,
		Discovery:    routingdiscovery,
		PubSub:       pubsubhandler,
		ChatroomName: chatroomname,
	}
	
	log.Printf("Host libp2p protocols: %s", strings.Join(nodehost.Mux().Protocols(), ", "))
	log.Printf("Connected to libp2p network with peerID %s listening on %v", p2phost.Host.ID().Pretty(), p2phost.Host.Addrs())

	return p2phost
}

// A method of P2P to connect to service peers.
// This method uses the Advertise() functionality of the Peer Discovery Service
// to advertise the service and then discovers all peers advertising the same.
// The peer discovery is handled by a go-routine that will read from a channel
// of peer address information until the peer channel closes
func (p2p *P2P) AdvertiseConnect() {
	ttl, err := p2p.Discovery.Advertise(p2p.Ctx, p2p.ChatroomName)
	if err != nil {
		log.Fatalf("Failed to Advertise Service! %v", err)
	}
	log.Printf("Advertised the %s Service.", p2p.ChatroomName)
	// Sleep to give time for the advertisment to propogate
	time.Sleep(time.Second * 5)
	log.Printf("Service Time-to-Live is %s", ttl)

	peerchan, err := p2p.Discovery.FindPeers(p2p.Ctx, p2p.ChatroomName)
	if err != nil {
		log.Fatalf("P2P Peer Discovery Failed! %v", err)
	}
	log.Printf("Discovered Service Peers.")

	go handlePeerDiscovery(p2p.Host, peerchan)
	log.Printf("Started Peer Connection Handler.")
}

// A method of P2P to connect to service peers.
// This method uses the Provide() functionality of the Kademlia DHT directly to announce
// the ability to provide the service and then discovers all peers that provide the same.
// The peer discovery is handled by a go-routine that will read from a channel
// of peer address information until the peer channel closes
func (p2p *P2P) AnnounceConnect() {
	// Generate the Service CID
	cidvalue := generateCID(p2p.ChatroomName)
	log.Printf("Generated the Service CID.")

	// Announce that this host can provide the service CID
	err := p2p.KadDHT.Provide(p2p.Ctx, cidvalue, true)
	if err != nil {
		log.Fatalf("Failed to Announce Service CID! %v", err)
	}
	log.Printf("Announced the %s Service.", p2p.ChatroomName)
	// Sleep to give time for the advertisment to propogate
	time.Sleep(time.Second * 5)

	peerchan := p2p.KadDHT.FindProvidersAsync(p2p.Ctx, cidvalue, 0)
	log.Printf("Discovered Service Peers.")

	go handlePeerDiscovery(p2p.Host, peerchan)
	log.Printf("Started Peer Connection Handler.")
}

func bootstrapPeers(addrs []string) []peer.AddrInfo {
	if len(addrs) == 0 {
		return dht.GetDefaultBootstrapPeerAddrInfos()
	}
	var mas []multiaddr.Multiaddr
	for _, s := range addrs {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {panic(err)}
		mas = append(mas,ma)
	}
	
	ds := make([]peer.AddrInfo, 0, len(mas))
	for i := range mas {
		info, err := peer.AddrInfoFromP2pAddr(mas[i])
		if err != nil {
			log.Printf("failed to convert bootstrapper address %v to peer addr info %v", mas[i].String(), err)
			continue
		}
		ds = append(ds, *info)
	}

	return ds
}

// Using a DHT is probably overkill for this application, where we will have few peers and they will not be online very long
func setupHostAndDHT(ctx context.Context, bootstrapaddrs []string, listenaddrs []string, me user.Me) (host.Host, *dht.IpfsDHT) {
	var err error
	// If bootstrapaddrs is empty, then we use the default public libp2p bootstrap peers
	bootstrappeers := bootstrapPeers(bootstrapaddrs)

	// The inner function will set kaddht var when it runs during libp2p.New
	var kaddht *dht.IpfsDHT
	routing := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		kaddht, err = dht.New(
			ctx, 
			h, 
			dht.Mode(dht.ModeAutoServer), 
			dht.BootstrapPeers(bootstrappeers...),
		)
		return kaddht, err
	})

	// libp2p defaults are /ip4/0.0.0.0/tcp/0, /ip6/::/tcp/0, enable relay, /yamux/1.0.0, /mplex/6.7.0, tls, noise, tcp, ws, empty peerstore		
	libhost, err := libp2p.New(
		routing,
		libp2p.ConnectionManager(connmgr.NewConnManager(50, 100, time.Minute)),
		libp2p.Identity(me.IdentPrivKey),
		libp2p.NATPortMap(), // attempts to use UPNP to open a port
		libp2p.EnableAutoRelay(),
	)
	if err != nil {
		log.Fatalf("Failed to Create the P2P Host! %v", err)
	}

	return libhost, kaddht
}

// A function that generates a PubSub Handler object and returns it
// Requires a node host and a routing discovery service.
func setupPubSub(ctx context.Context, nodehost host.Host, routingdiscovery *discovery.RoutingDiscovery) *pubsub.PubSub {
	// TODO investigate using custom protocol so we dont join the default public gossip net?
	// https://github.com/libp2p/go-libp2p-pubsub/pull/413/files
	// customsub := protocol.ID("customsub/1.0.0")
	// var GossipSubDefaultProtocols = []protocol.ID{GossipSubID_v11, GossipSubID_v10, FloodSubID}
	// protos := []protocol.ID{customsub, FloodSubID}
	// features := func(feat GossipSubFeature, proto protocol.ID) bool {
	// 	if proto == customsub {
	// 		return true
	// 	}

	// 	return false
	// }
	// pubsubhandler, err := pubsub.NewGossipSub(ctx, nodehost, [pubsub.WithDiscovery(routingdiscovery), pubsub.WithGossipSubProtocols(protos, features)])
  
	pubsubhandler, err := pubsub.NewGossipSub(ctx, nodehost, pubsub.WithDiscovery(routingdiscovery))
	if err != nil {
		log.Fatalf("PubSub Handler Creation Failed! %v", err)
	}

	return pubsubhandler
}

// A function that bootstraps a given Kademlia DHT to satisfy the IPFS router
// interface and connects to all the bootstrap peers provided
func bootstrapDHT(ctx context.Context, nodehost host.Host, kaddht *dht.IpfsDHT, bootstrapaddrs []string) {
	bootstrappeers := bootstrapPeers(bootstrapaddrs)

	// Bootstrap the DHT to satisfy the IPFS Router interface
	if err := kaddht.Bootstrap(ctx); err != nil {
		log.Fatalf("Failed to Bootstrap the Kademlia! %v", err)
	}

	log.Printf("Set the Kademlia DHT into Bootstrap Mode.")

	var wg sync.WaitGroup

	var connectedbootpeers int
	var totalbootpeers int

	// for _, peeraddr := range dht.DefaultBootstrapPeers {
	// if len(bootstrappeers) == 0 {
	// 	bootstrappeers = dht.GetDefaultBootstrapPeerAddrInfos()
	// }

	for _, peeraddr := range bootstrappeers {
		p := peeraddr
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := nodehost.Connect(ctx, p); err != nil {
				totalbootpeers++
			} else {
				connectedbootpeers++
				totalbootpeers++
			}
		}()
	}

	wg.Wait()

	log.Printf("Connected to %d out of %d Bootstrap Peers.", connectedbootpeers, totalbootpeers)
	if connectedbootpeers < 1 {
		fmt.Printf("\n\nFailed to connect to any of the bootstrap peers!\n%v \nExiting.\n", bootstrapaddrs)
		log.Fatalf("\n\nFailed to connect to any of the bootstrap peers!\n%v \nExiting.\n", bootstrapaddrs)
	}
}

// A function that connects the given host to all peers recieved from a
// channel of peer address information. Meant to be started as a go routine.
func handlePeerDiscovery(nodehost host.Host, peerchan <-chan peer.AddrInfo) {
	// Iterate over the peer channel
	for peer := range peerchan {
		// Ignore if the discovered peer is the host itself
		if peer.ID == nodehost.ID() {
			continue
		}

		log.Printf("handlePeerDiscovery %s", peer.String())

		// Connect to the peer
		err := nodehost.Connect(context.Background(), peer)
		log.Printf("Connect to peer: %v  %v", peer.String(), err)
	}
}

// A function that generates a CID object for a given string and returns it.
// Uses SHA256 to hash the string and generate a multihash from it.
// The mulithash is then base58 encoded and then used to create the CID
func generateCID(namestring string) cid.Cid {
	// Hash the service content ID with SHA256
	hash := sha256.Sum256([]byte(namestring))
	// Append the hash with the hashing codec ID for SHA2-256 (0x12),
	// the digest size (0x20) and the hash of the service content ID
	finalhash := append([]byte{0x12, 0x20}, hash[:]...)
	// Encode the fullhash to Base58
	b58string := base58.Encode(finalhash)

	// Generate a Multihash from the base58 string
	mulhash, err := multihash.FromB58String(string(b58string))
	if err != nil {
		log.Fatalf("Failed to Generate Service CID! %v", err)
	}

	// Generate a CID from the Multihash
	cidvalue := cid.NewCidV1(12, mulhash)
	// Return the CID
	return cidvalue
}
