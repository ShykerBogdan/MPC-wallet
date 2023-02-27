package chat

import (
	"github.com/johnthethird/thresher/network"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
)

// A Network that supports chat messages as well as protocol messages for the multi-party-sig (MPS) library
type chatNetwork struct {
	inboundProtocol chan *protocol.Message
	outboundProtocol chan *protocol.Message
	inboundChat chan chatmessage
	outboundChat chan chatmessage
}

func NewNetwork(cr *ChatRoom) network.Network {
	ipc := cr.InboundProtocol
	opc := cr.OutboundProtocol
	icc:= cr.InboundChat
	occ:= cr.OutboundChat
	n := &chatNetwork{
		inboundProtocol: ipc,
		outboundProtocol: opc,
		inboundChat: icc,
		outboundChat: occ,
	}
	return n
}

func (c *chatNetwork) Next(id party.ID) <-chan *protocol.Message {
	return c.inboundProtocol
}

func (c *chatNetwork) Send(msg *protocol.Message) {
	protmsg := chatmessage{Type: messageTypeProtocol, ProtocolMessage: msg}
	c.outboundChat <- protmsg
}

