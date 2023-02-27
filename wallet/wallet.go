package wallet

import (
	mpsconfig "github.com/taurusgroup/multi-party-sig/protocols/cmp/config"
)

// TODO Figure out how to deal with different blockchain libs wanting different
// types for these func calls. Do I need to build out a facade that sits in front
// of them all? Punting for now.
type Wallet interface {
	// keydata from the MPS keygen protocol
	Initialize(keydata []byte)
	GetUnwrappedKeyData() mpsconfig.Config
	// User-supplied name of this wallet
	GetName() string
	SetName(n string)
	// GetAddress returns one of the addresses this wallet manages. For now this is a single 
	// addr but let's try to support HD addresses that derive from the multisig
	GetFormattedAddress() string
	// Get the tx fee for the blockchain
	GetTxFee() (amount uint64)
	Balance(assetID interface{}) uint64
	// Create an unsigned transaction suitable for signing
	CreateTx(assetID interface{}, amount uint64, destAddr interface{}, memo string) (interface{}, error)
	// Fetch UTXOs over the network
	FetchUTXOs() error

	// Directly add a UTXO to this wallet, if it is spendable
	AddUTXO(utxo interface{})
	RemoveUTXO(utxo interface{})
}