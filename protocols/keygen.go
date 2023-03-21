package protocols

import (
	"log"

	"github.com/fxamacker/cbor/v2"
	"github.com/shykerbogdan/mpc-wallet/network"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet"
	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/pool"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/protocols/cmp"
	"github.com/taurusgroup/multi-party-sig/protocols/cmp/config"
)

func RunKeygen(w *ethwallet.Wallet, net network.Network) error {
	selfid := w.Me.PartyID()
	allids := w.AllPartyIDs()
	threshold := w.Threshold
	log.Printf("Starting Keygen protocol - selfid: %v, allids: %v threshold: %v", selfid, allids, threshold)

	pl := pool.NewPool(0)
	defer pl.TearDown()

	h, err := protocol.NewMultiHandler(cmp.Keygen(curve.Secp256k1{}, selfid, allids, threshold, pl), nil)
	if err != nil {
		return err
	}

	handlerLoop(selfid, h, net)

	r, err := h.Result()
	if err != nil {
		return err
	}

	log.Print("Keygen protocol complete")

	c := r.(*config.Config)

	log.Printf("KeyData result: %v", c)

	cb, err := cbor.Marshal(c)
	log.Printf("KeyData marshal: %v", cb)

	if err != nil {
		return err
	}

	w.Initialize(cb)

	return nil
}
