package protocols

import (
	"errors"
	"log"

	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/shykerbogdan/mpc-wallet/network"
	"github.com/shykerbogdan/mpc-wallet/user"
	"github.com/shykerbogdan/mpc-wallet/wallet/avmwallet"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/pool"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/protocols/cmp"
)

func RunSign(w *avmwallet.Wallet, msghash []byte, signers []user.User, net network.Network) (*mpsecdsa.Signature, error) {
	pl := pool.NewPool(0)
	defer pl.TearDown()

	partyIDs := party.IDSlice{}
	for _, u := range signers {
		partyIDs = append(partyIDs, u.PartyID())
	}

	cfg := w.GetUnwrappedKeyData()

	cb58MsgHash, _ := formatting.EncodeWithChecksum(formatting.CB58, msghash)
	log.Printf("Starting Sign protocol - wallet: %s, msghash(cb58): %s, partyids: %v", w.GetName(), cb58MsgHash, partyIDs)

	// TODO do we really need a session ID? Or is nil OK?
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

	avaSignature, err := w.MpsSigToAvaSig(msghash, signature)
	if err != nil {
		return nil, err
	}

	avaSignatureCB58, _ := formatting.EncodeWithChecksum(formatting.CB58, avaSignature)

	if !signature.Verify(cfg.PublicPoint(), msghash) {
		msg := "failed to verify cmp signature"
		log.Print(msg)
		return nil, errors.New(msg)
	}

	if !w.VerifyHash(msghash, signature) {
		msg := "failed to verify avalanche signature"
		log.Print(msg)
		return nil, errors.New(msg)
	}

	log.Printf("Sign protocol complete, avaSignatureCB58: %s", avaSignatureCB58)

	return signature, nil
}
