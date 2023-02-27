package avmwallet

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

var errCantSpend = errors.New("unable to spend this UTXO")


// Keychain is a collection of multisig public keys that can be used to spend outputs
// TODO see MPS DeriveBIP32, derives a sharing of the ith child of the consortium signing key.
// TODO what about asset IDs? Should handling of those be in this struct?
type Keychain struct {
	Addrs ids.ShortSet
}

// NewKeychain returns a new, empty, keychain
func NewKeychain() *Keychain {
	return &Keychain{}
}

// Add a new key to the key chain
func (kc *Keychain) Add(key *crypto.PublicKeySECP256K1R) {
	addr := key.Address()
	kc.Addrs.Add(addr)
}

// Spend attempts to create an input from an output
func (kc *Keychain) Spend(out verify.Verifiable, time uint64) (verify.Verifiable, error) {
	switch out := out.(type) {
	// TODO support minting, etc
	// case *secp256k1fx.MintOutput:
	// 	if able := kc.Match(&out.OutputOwners, time); able {
	// 		return &secp256k1fx.Input{
	// 			SigIndices: []uint32{0},
	// 		}, nil
	// 	}
	// 	return nil, errCantSpend
	case *secp256k1fx.TransferOutput:
		if able := kc.Match(&out.OutputOwners, time); able {
			return &secp256k1fx.TransferInput{
				Amt: out.Amt,
				Input: secp256k1fx.Input{
					// SigIndices is a list of unique ints that define the private keys that are being used to spend the UTXO. Each UTXO has an array of addresses that can spend the UTXO. Each int represents the index in this address array that will sign this transaction. The array must be sorted low to high.
					// So, this should always be zero since all UTXOs for our wallet will only ever have 1 sig required (as far as avalanche is concerned)
					SigIndices: []uint32{0},
				},
			}, nil
		}
		return nil, errCantSpend
	}
	return nil, fmt.Errorf("can't spend UTXO because it is unexpected type %T", out)
}

// Match only outputs that have one owner (the multi sig key)
func (kc *Keychain) Match(owners *secp256k1fx.OutputOwners, time uint64) bool {
	if time < owners.Locktime || owners.Threshold != 1 || len(owners.Addrs) != 1 || !kc.Addrs.Contains(owners.Addrs[0]) {
		return false
	}
	return true
}

