package protocols

import (
	"log"

	"github.com/shykerbogdan/mpc-wallet/network"
	"github.com/shykerbogdan/mpc-wallet/user"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/pool"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/protocols/cmp"
)

func RunSign(w *ethwallet.Wallet, msghash []byte, signers []user.User, net network.Network) (*mpsecdsa.Signature, error) {
	pl := pool.NewPool(0)
	defer pl.TearDown()

	partyIDs := party.IDSlice{}
	for _, u := range signers {
		partyIDs = append(partyIDs, u.PartyID())
	}

	cfg := w.GetUnwrappedKeyData()

	
	h, err := protocol.NewMultiHandler(cmp.Sign(&cfg, partyIDs, msghash, pl), nil)
	if err != nil {
		return nil, err
	}

	handlerLoop(cfg.ID, h, net)

	signResult, err := h.Result()
	if err != nil {
		return nil, err
	}

	signature := signResult.(*mpsecdsa.Signature)

	log.Printf("MPC lib signature: %s", signature)

	return signature, nil
}
