package network

import (
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
)

type Network interface {
	Send(msg *protocol.Message)
	Next(id party.ID) <-chan *protocol.Message
}

