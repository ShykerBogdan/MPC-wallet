package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/shykerbogdan/mpc-wallet/config"
	"github.com/shykerbogdan/mpc-wallet/protocols"
	"github.com/shykerbogdan/mpc-wallet/user"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
)

const (
	// Drop participant from room if we dont hear advertise msgs from them for this long
	participantTTL = time.Second * 30
)

// messageType enumerates the possible types of pubsub room messages.
type messageType string

const (
	// messageTypeChatMessage is published when a new chat message is sent from the node.
	messageTypeChatMessage messageType = "chat.message"

	// messageTypeAdvertise is published periodically to indicate a node is still connected to a room.
	messageTypeAdvertise messageType = "chat.advertise"

	// messageTypeProtocol is published when a new protocol message is contained within the chat message
	messageTypeProtocol messageType = "chat.protocol"

	messageTypeStartKeygen messageType = "chat.startkeygen"
	messageTypeStartSign   messageType = "chat.startsign"
	messageTypeStartSendTx messageType = "chat.startsendtx"
)

// TODO Stuffing everything into one msg struct for now, better way?
type chatmessage struct {
	Type             messageType       `json:"type"` // chat, command, protocol
	SenderID         string            `json:"senderid"`
	SenderName       string            `json:"sendername"`
	StartKeygen      startkeygencmd    `json:"startkeygen,omitempty"`
	StartSign        startsigncmd      `json:"startsign,omitempty"`
	StartSendTx      startsendtxcmd    `json:"startsendtx,omitempty"`
	UserMessage      string            `json:"usermessage,omitempty"`
	ProtocolMessage  *protocol.Message `json:"protmessage,omitempty"`
	AdvertiseMessage user.User         `json:"advmsg,omitempty"`
	EventMessage     string            `json:"evtmsg,omitempty"`
}

type participant struct {
	user.User
	verified bool
	peerid   peer.ID
	ttl      time.Duration
	addedAt  time.Time
}

type startkeygencmd struct {
	Name      string
	Threshold int
	Signers   []user.User
}

type startsigncmd struct {
	Name    string
	Message string
	Signers []user.User
}

type startsendtxcmd struct {
	Name     string
	Amount   uint64
	DestAddr string
	Memo     string
	Signers  []user.User
}

// A structure that represents a chat log displayed locally and not published
type logLevelType string

const (
	logLevelDebug logLevelType = "🚧 "
	logLevelInfo  logLevelType = "ℹ️ "
	logLevelError logLevelType = "❗️ "
)

type chatlog struct {
	level logLevelType
	msg   string
}

// A structure that represents a PubSub Chat Room
type ChatRoom struct {
	// Represents the P2P Host for the ChatRoom
	Host *P2P

	InboundChat          chan chatmessage
	OutboundChat         chan chatmessage
	InboundProtocolStart chan chatmessage
	InboundProtocol      chan *protocol.Message
	OutboundProtocol     chan *protocol.Message
	Logs                 chan chatlog

	cfg *config.AppConfig

	peerid       peer.ID
	participants map[peer.ID]*participant

	mutex sync.RWMutex

	// Represents the chat room lifecycle context
	psctx context.Context
	// Represents the chat room lifecycle cancellation function
	pscancel context.CancelFunc
	// Represents the PubSub Topic of the ChatRoom
	pstopic *pubsub.Topic
	// Represents the PubSub Subscription for the topic
	psub *pubsub.Subscription
}

// A constructor function that generates and returns a new
// ChatRoom for a given P2PHost, username and roomname
func JoinChatRoom(p2phost *P2P, cfg *config.AppConfig) (*ChatRoom, error) {

	// Create a PubSub topic with the Project name
	topic, err := p2phost.PubSub.Join(cfg.Project)
	if err != nil {
		return nil, err
	}

	// Subscribe to the PubSub topic
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	// Create cancellable context
	pubsubctx, cancel := context.WithCancel(context.Background())

	const channel_size = 10
	// Create a ChatRoom object
	chatroom := &ChatRoom{
		Host: p2phost,

		InboundChat:          make(chan chatmessage, channel_size),
		OutboundChat:         make(chan chatmessage, channel_size),
		InboundProtocolStart: make(chan chatmessage, channel_size),
		InboundProtocol:      make(chan *protocol.Message, channel_size),
		OutboundProtocol:     make(chan *protocol.Message, channel_size),
		Logs:                 make(chan chatlog, channel_size),

		psctx:    pubsubctx,
		pscancel: cancel,
		pstopic:  topic,
		psub:     sub,

		cfg:          cfg,
		peerid:       p2phost.Host.ID(),
		participants: make(map[peer.ID]*participant),
	}

	go chatroom.SubLoop()
	go chatroom.PubLoop()
	go chatroom.advertiseLoop()
	go chatroom.refreshParticipantsLoop()

	return chatroom, nil
}

// A method of ChatRoom that publishes a chatmessage
// to the PubSub topic until the pubsub context closes
func (cr *ChatRoom) PubLoop() {
	for {
		select {
		case <-cr.psctx.Done():
			return

		case message := <-cr.OutboundChat:
			messagebytes, err := json.Marshal(message)
			// log.Printf("DEBUG publish json: %+v", string(messagebytes))
			if err != nil {
				cr.Logs <- chatlog{level: logLevelError, msg: "could not marshal JSON"}
				continue
			}

			// Publish the message to the topic
			err = cr.pstopic.Publish(cr.psctx, messagebytes)
			if err != nil {
				cr.Logs <- chatlog{level: logLevelError, msg: "could not publish to topic"}
				continue
			}
		}
	}
}

// A method of ChatRoom that continously reads from the subscription
// until either the subscription or pubsub context closes.
// The recieved message is parsed sent into the inbound channel
func (cr *ChatRoom) SubLoop() {
	// Start loop
	for {
		select {
		case <-cr.psctx.Done():
			return

		default:
			// Read a message from the subscription
			message, err := cr.psub.Next(cr.psctx)
			if err != nil {
				// Close the messages queue (subscription has closed)
				close(cr.InboundChat)
				cr.Logs <- chatlog{level: logLevelError, msg: "subscription has closed"}
				return
			}

			// if message is from self then do nothing
			if message.ReceivedFrom == cr.peerid {
				continue
			}

			cm := &chatmessage{}
			err = json.Unmarshal(message.Data, cm)
			if err != nil {
				cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("could not unmarshal chat message: %v", err)}
				continue
			}

			switch cm.Type {
			case messageTypeChatMessage:
				cr.InboundChat <- *cm
			case messageTypeStartKeygen:
				if cr.doSignersIncludeMe(cm.StartKeygen.Signers) {
					cr.InboundProtocolStart <- *cm
				}
			case messageTypeStartSign:
				if cr.doSignersIncludeMe(cm.StartSign.Signers) {
					cr.InboundProtocolStart <- *cm
				}
			case messageTypeStartSendTx:
				if cr.doSignersIncludeMe(cm.StartSendTx.Signers) {
					cr.InboundProtocolStart <- *cm
				}
			case messageTypeProtocol:
				if cr.isProtocolMsgForMe(cm) {
					cr.Logs <- chatlog{level: logLevelDebug, msg: fmt.Sprintf("Processing mpc-cmp protocol msg round %v...", cm.ProtocolMessage.RoundNumber)}
					cr.InboundProtocol <- cm.ProtocolMessage
				}
			case messageTypeAdvertise:
				cr.AddParticipant(message.ReceivedFrom, cm.AdvertiseMessage)
			default:
				cr.Logs <- chatlog{level: logLevelInfo, msg: fmt.Sprintf("received unknown msg type %v", cm.Type)}
			}
		}
	}
}

func (cr *ChatRoom) isProtocolMsgForMe(cm *chatmessage) bool {
	return cm.Type == messageTypeProtocol && cm.ProtocolMessage != nil && cm.ProtocolMessage.IsFor(cr.cfg.Me.PartyID())
}

func (cr *ChatRoom) doSignersIncludeMe(signers []user.User) bool {
	for _, u := range signers {
		if u.Nick == cr.cfg.Me.Nick {
			return true
		}
	}
	return false
}

func (cr *ChatRoom) runProtocolKeygen(walletname string, threshold int, signers []user.User) {
	net := NewNetwork(cr)
	wallet := cr.cfg.NewEmptyWallet(walletname, threshold, signers)
	err := protocols.RunKeygen(wallet, net)
	if err != nil {
		log.Fatalf("Error running keygen protocol: %v", err)
	}

	err = cr.cfg.AddWallet(wallet)
	if err != nil {
		log.Fatalf("Error saving keygen protocol result to wallet: %v", err)
	}

	cr.Logs <- chatlog{level: logLevelInfo, msg: fmt.Sprintf("Wallet '%s' has been generated.", walletname)}
}

func (cr *ChatRoom) runProtocolSign(walletname string, msghash []byte, signers []user.User) []byte {
	net := NewNetwork(cr)
	wallet := cr.cfg.FindWallet(walletname)
	sig, err := protocols.RunSign(wallet, msghash, signers, net)
	if err != nil {
		log.Fatalf("Error running signing protocol: %v", err)
	}

	avasig, err := wallet.MpsSigToAvaSig(msghash, sig)
	if err != nil {
		log.Fatalf("Error recovering avasig: %v", err)
	}

	return avasig
}

func (cr *ChatRoom) runProtocolSendTx(walletname string, destaddr string, amount uint64, memo string, signers []user.User) {
	w := cr.cfg.FindWallet(walletname)
	err := w.FetchUTXOs()
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error fetching wallet balance %v", err)}
		return
	}

	_, _, b, err := formatting.ParseAddress(destaddr)
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error parsing dest addr %s: %v", destaddr, err)}
		return
	}
	destid, err := ids.ToShortID(b)
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error parsing dest addr to id %s: %v", destaddr, err)}
		return
	}

	tx, err := w.CreateTx(w.Config.AssetID, amount, destid, memo)
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error CreateTx %v", err)}
		return
	}

	unsignedBytes, err := w.GetUnsignedBytes(&tx.UnsignedTx)
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error GetUnsignedBytes %v", err)}
		return
	}
	msgHash := hashing.ComputeHash256(unsignedBytes)

	avasig := cr.runProtocolSign(walletname, msgHash, signers)

	avasigcb58, _ := formatting.EncodeWithChecksum(formatting.CB58, avasig)
	log.Printf("avasigcb58: %v", avasigcb58)

	// TODO move all this into wallet

	for range tx.InputUTXOs() {
		cred := &secp256k1fx.Credential{
			Sigs: make([][crypto.SECP256K1RSigLen]byte, 1),
		}
		copy(cred.Sigs[0][:], avasig)
		tx.Creds = append(tx.Creds, &avm.FxCredential{Verifiable: cred})
	}

	signedBytes, err := w.Marshal(tx)
	if err != nil {
		log.Fatalf("problem marshaling transaction: %v", err)
	}

	tx.Initialize(unsignedBytes, signedBytes)

	txcb58, _ := formatting.EncodeWithChecksum(formatting.CB58, tx.Bytes())
	log.Printf("txcb58: %v", txcb58)
	log.Print(w.FormatIssueTxAsCurl(txcb58))

	txID, err := w.IssueTx(tx.Bytes())
	if err != nil {
		cr.Logs <- chatlog{level: logLevelError, msg: fmt.Sprintf("Error issuing tx: %s", err)}
		return
	}
	cr.Logs <- chatlog{level: logLevelInfo, msg: fmt.Sprintf("Issued txid: %s", txID.String())}

	result := w.ConfirmTx(txID)
	url := w.FormatTxURL(txID)

	if result {
		msg := fmt.Sprintf("[blue]🎉 Transaction Confirmed![-] %s", url)
		cr.Logs <- chatlog{level: logLevelInfo, msg: msg}
		cr.OutboundChat <- chatmessage{Type: messageTypeChatMessage, SenderName: cr.cfg.Me.Nick, UserMessage: msg}
	} else {
		msg := fmt.Sprintf("[red]😱 Transaction did not confirm![-] %s", url)
		cr.Logs <- chatlog{level: logLevelError, msg: msg}
		cr.OutboundChat <- chatmessage{Type: messageTypeChatMessage, SenderName: cr.cfg.Me.Nick, UserMessage: msg}
	}
}

func (cr *ChatRoom) AddParticipant(peerid peer.ID, u user.User) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	if cr.cfg.Me.Nick == u.Nick {
		return
	}

	p := participant{
		peerid:   peerid,
		User:     u,
		verified: u.IsVerified(),
		ttl:      participantTTL,
		addedAt:  time.Now(),
	}

	_, exists := cr.participants[peerid]

	cr.participants[peerid] = &p

	if !exists {
		msg := fmt.Sprintf("%s has joined the chat.", cr.participants[peerid].Nick)
		// TODO SECURITY figure out how to verify the nick somehow, like ensure the peer id is same as in the wallet?
		// cr.Cfg.IsVerified(nick, peerid) can look through all wallets we have and ensure the nick matches the peerid?
		// if u.IsVerified() {
		// 	msg = fmt.Sprintf("%s has joined the chat.", cr.participants[peerid].Nick)
		// } else {
		// 	msg = fmt.Sprintf("ALERT %s has joined the chat with an unverified nick %s", cr.participants[peerid].Nick, cr.participants[peerid].Address)
		// }
		cr.Logs <- chatlog{level: logLevelInfo, msg: msg}
	}
}

func (cr *ChatRoom) ParticipantList() []*participant {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	arr := []*participant{}
	for _, v := range cr.participants {
		arr = append(arr, v)
	}

	sort.Slice(arr, func(i, j int) bool {
		return arr[i].Nick < arr[j].Nick
	})

	return arr
}

func (cr *ChatRoom) Exit() {
	defer cr.pscancel()
	cr.psub.Cancel()
	cr.pstopic.Close()
}

// Publish our nick to the channel participants
func (cr *ChatRoom) advertiseLoop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		<-ticker.C
		cr.OutboundChat <- chatmessage{Type: messageTypeAdvertise, SenderName: cr.cfg.Me.Nick, AdvertiseMessage: cr.cfg.Me.User}
	}
}

func (cr *ChatRoom) refreshParticipantsLoop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		<-ticker.C
		func() {
			cr.mutex.Lock()
			defer cr.mutex.Unlock()

			for peerID, participant := range cr.participants {
				if time.Since(participant.addedAt) <= participant.ttl {
					continue
				}

				cr.Logs <- chatlog{level: logLevelInfo, msg: fmt.Sprintf("%s has left the chat.", cr.participants[peerID].Nick)}
				// participant ttl expired
				delete(cr.participants, peerID)
			}
		}()
	}
}
