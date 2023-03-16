package avmwallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	stdmath "math"

	"github.com/shykerbogdan/mpc-wallet/constants"
	"github.com/shykerbogdan/mpc-wallet/user"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	avacrypto "github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/decred/dcrd/dcrec/secp256k1/v3"
	"github.com/fxamacker/cbor/v2"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	mpsconfig "github.com/taurusgroup/multi-party-sig/protocols/cmp/config"
)

const (
	codecVersion = 0
	txFee        = units.MilliAvax // default for Fuji and Mainnet
	apiTimeout   = "30s"
)

type Asset struct {
	AssetID      ids.ID
	Name         string
	Symbol       string
	Denomination uint8
}

// Wallet is a holder for keys and UTXOs for the blockchain.
// For now we use just one pub/priv key, maybe use BIP32 to enhance privacy?
type Wallet struct {
	// User-supplied name for this wallet
	Name      string
	Threshold int
	Me        user.User
	Others    []user.User

	// Raw config.Config struct we get from the MPS Keygen protocol
	// This is a SECRET so figure out best way to protect it
	KeyData []byte
	// Public address computed from the MPS config and stored here so it shows up in the persisted JSON for reference
	Address string
	// Config params for a blockchain, i.e. Avax Fuji, etc
	Config constants.ChainConfig

	CreatedAt time.Time

	// Mapping from utxoIDs to UTXOs
	utxoSet *UTXOSet

	// For now we use just one pub/priv key, maybe use BIP32 to enhance privacy?
	keychain *Keychain
	// Mapping of asset IDs to balances
	balance map[ids.ID]uint64
	// Set of unique asset ids contained in the utxos
	assets map[ids.ID]Asset

	txFee uint64

	// txs []*avm.Tx

	isFetching bool
	codec      codec.Manager
	clock      timer.Clock
	mutex      sync.Mutex
}

// NewWallet returns a new Avalanche Wallet
func NewEmptyWallet(network string, name string, threshold int, me user.User, others []user.User) *Wallet {
	w := &Wallet{
		Name:      name,
		Threshold: threshold,
		Me:        me,
		Others:    others,
		CreatedAt: time.Now().UTC(),
		keychain:  NewKeychain(),
		balance:   map[ids.ID]uint64{},
		assets:    map[ids.ID]Asset{},
		txFee:     txFee,
	}

	switch network {
	case "goerli":
		w.Config = constants.GoerliConfig
	case "fuji":
		w.Config = constants.AvmFujiConfig
	case "mainnet":
		w.Config = constants.AvmMainnetConfig
	}

	return w
}

// Do the funky chicken to unmarshal then init the struct
// TODO is this really best way init an unmarshaled struct?
func (w *Wallet) UnmarshalJSON(data []byte) error {
	type Alias Wallet
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(w),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	w.Initialize(w.KeyData)
	return nil
}

func (w *Wallet) Initialize(keydata []byte) {
	w.utxoSet = &UTXOSet{}
	w.balance = map[ids.ID]uint64{}
	w.assets = map[ids.ID]Asset{}
	w.KeyData = keydata
	w.keychain = NewKeychain()
	w.keychain.Add(w.PublicKeyAvm().(*avacrypto.PublicKeySECP256K1R))

	// Store in the struct so it gets serialized for easy reference
	w.Address = w.GetFormattedAddress()
}

func (w *Wallet) GetName() string     { return w.Name }
func (w *Wallet) SetName(name string) { w.Name = name }

// All signers excluding me
func (w *Wallet) OtherPartyIDs() party.IDSlice {
	var list []party.ID
	for _, s := range w.Others {
		list = append(list, s.PartyID())
	}
	return party.NewIDSlice(list)
}

// All signers as partyIDs (required by the MSP library)
func (w *Wallet) AllPartyIDs() party.IDSlice {
	var list []party.ID
	list = append(list, w.Me.PartyID())

	for _, s := range w.Others {
		list = append(list, s.PartyID())
	}
	return party.NewIDSlice(list)
}

// All signers as an array of string nicknames
func (w *Wallet) AllPartyNicks() []string {
	var list []string
	list = append(list, w.Me.Nick)

	for _, s := range w.Others {
		list = append(list, s.Nick)
	}

	sort.Strings(list)

	return list
}

// Unmarshal the MPS config which contains the key data
func (w *Wallet) GetUnwrappedKeyData() mpsconfig.Config {
	// TODO cache this
	c := mpsconfig.EmptyConfig(curve.Secp256k1{})
	err := cbor.Unmarshal(w.KeyData, c)
	if err != nil {
		log.Fatalf("GetUnwrappedKeyData error %v", err)
	} //TODO
	return *c
}

// From the MSP key data, convert to an Avalanche public key
func (w *Wallet) PublicKeyAvm() avacrypto.PublicKey {
	kd := w.GetUnwrappedKeyData()
	ppb, err := kd.PublicPoint().MarshalBinary()
	if err != nil {
		panic(err)
	} //TODO
	f := avacrypto.FactorySECP256K1R{}
	avapk, err := f.ToPublicKey(ppb)
	if err != nil {
		panic(err)
	} //TODO
	return avapk
}

func (w *Wallet) PublicKeyMpsPoint() curve.Point {
	kd := w.GetUnwrappedKeyData()
	ppb := kd.PublicPoint()
	return ppb
}

// Codec returns the codec used for serialization
func (w *Wallet) Codec() codec.Manager {
	if w.codec != nil {
		return w.codec
	}

	c := linearcodec.NewDefault()
	m := codec.NewDefaultManager()
	errs := wrappers.Errs{}
	errs.Add(
		c.RegisterType(&avm.BaseTx{}),
		c.RegisterType(&avm.CreateAssetTx{}),
		c.RegisterType(&avm.OperationTx{}),
		c.RegisterType(&avm.ImportTx{}),
		c.RegisterType(&avm.ExportTx{}),
		c.RegisterType(&secp256k1fx.TransferInput{}),
		c.RegisterType(&secp256k1fx.MintOutput{}),
		c.RegisterType(&secp256k1fx.TransferOutput{}),
		c.RegisterType(&secp256k1fx.MintOperation{}),
		c.RegisterType(&secp256k1fx.Credential{}),
		m.RegisterCodec(codecVersion, c),
	)

	w.codec = m
	return m
}

func (w *Wallet) Marshal(source interface{}) (destination []byte, err error) {
	return w.Codec().Marshal(codecVersion, source)
}

func (w *Wallet) Unmarshal(source []byte) (destination interface{}, err error) {
	var x interface{}
	_, e := w.Codec().Unmarshal(source, &x)
	if e != nil {
		return x, e
	}
	return x, nil
}

// The string form of an Avalanche address (X-fuji1blahblah...)
func (w *Wallet) GetFormattedAddress() string {
	b := w.PublicKeyAvm().Address().Bytes()
	addr, err := formatting.FormatAddress(w.Config.ChainName, w.Config.NetworkName, b)
	if err != nil {
		panic("Fatal error converting multisig public key to Avax format")
	}

	return addr
}

func (w *Wallet) clearBalances() {
	w.utxoSet = &UTXOSet{}
	w.balance = map[ids.ID]uint64{}
}

// Is the wallet querying the network for UTXOs
func (w *Wallet) IsFetching() bool {
	return w.isFetching
}

// Query the network for UTXOs
// Run in a Go routine, as well as called directly
func (w *Wallet) FetchUTXOs() error {
	w.mutex.Lock()
	w.isFetching = true
	defer w.doneFetching()
	defer w.mutex.Unlock()

	d, _ := time.ParseDuration(apiTimeout)
	c := avm.NewClient(w.Config.RPCHostURL, w.Config.ChainName, d)
	// TODO handle pagination if more that 1024
	allUTXOBytes, _, err := c.GetUTXOs([]string{w.GetFormattedAddress()}, 1024, "", "")
	if err != nil {
		log.Printf("FetchUTXOs Error fetching: %v", err)
		return err
	}

	w.clearBalances()

	for _, ub := range allUTXOBytes {
		utxo := &avax.UTXO{}
		_, err = w.Codec().Unmarshal(ub, utxo)
		if err != nil {
			// TODO an error is probably an NFT, how to handle that properly?
			// log.Printf("FetchUTXOs Error marshaling %+v: %v", ub, err)
			// return err
		} else {
			w.addUTXO(utxo)
		}
	}

	// Now get AssetDescriptions for all unique assets we have
	for _, u := range w.utxoSet.UTXOs {
		if _, found := w.assets[u.AssetID()]; !found {
			adr, err := c.GetAssetDescription(u.AssetID().String())
			if err != nil {
				log.Printf("FetchUTXOs Error fetching asset description for assetID %s: %v", u.AssetID().String(), err)
			} else {
				a := Asset{
					AssetID:      adr.AssetID,
					Name:         adr.Name,
					Symbol:       adr.Symbol,
					Denomination: uint8(adr.Denomination),
				}
				w.assets[u.AssetID()] = a
			}
		}
	}

	return nil
}

func (w *Wallet) doneFetching() {
	w.isFetching = false
}

func (w *Wallet) IssueTx(txBytes []byte) (ids.ID, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	d, _ := time.ParseDuration(apiTimeout)
	c := avm.NewClient(w.Config.RPCHostURL, w.Config.ChainName, d)
	txID, err := c.IssueTx(txBytes)
	if err != nil {
		return ids.ID{}, err
	}
	return txID, nil
}

// Blocks until confirmed, 3 attempts with 3 second delay
func (w *Wallet) ConfirmTx(txID ids.ID) bool {
	attempts := 3
	delay, _ := time.ParseDuration("3s")
	d, _ := time.ParseDuration(apiTimeout)
	c := avm.NewClient(w.Config.RPCHostURL, w.Config.ChainName, d)
	result, _ := c.ConfirmTx(txID, attempts, delay)
	return result == choices.Accepted
}

// AddUTXO adds a new UTXO to this wallet if this wallet may spend it
// The UTXO's output must be an OutputPayment
func (w *Wallet) addUTXO(utxo *avax.UTXO) {
	out, ok := utxo.Out.(avax.TransferableOut)
	if !ok {
		log.Printf("AddUTXO problem with output %+v", *utxo)
		return
	}

	if _, err := w.keychain.Spend(out, stdmath.MaxUint64); err == nil {
		w.utxoSet.Put(utxo)
		if _, found := w.balance[utxo.AssetID()]; !found {
			w.balance[utxo.AssetID()] = 0
		}
		w.balance[utxo.AssetID()] += out.Amount()
	} else {
		log.Printf("addUTXO: Cant spend output: %+v", out)
	}
}

// // RemoveUTXO from this wallet
// func (w *Wallet) removeUTXO(utxoID ids.ID) {
// 	utxo := w.utxoSet.Get(utxoID)
// 	if utxo == nil {
// 		return
// 	}

// 	assetID := utxo.AssetID()
// 	// TODO can I use regular ol' minus here?
// 	newBalance := w.balance[assetID] - utxo.Out.(avax.TransferableOut).Amount()
// 	if newBalance == 0 {
// 		delete(w.balance, assetID)
// 	} else {
// 		w.balance[assetID] = newBalance
// 	}

// 	w.utxoSet.Remove(utxoID)
// }

// Balance returns the amount of the assets in this wallet
func (w *Wallet) Balance(assetID ids.ID) uint64 {
	return w.balance[assetID]
}

func (w *Wallet) GetBalances() map[ids.ID]uint64 {
	// TODO should return a copy?
	return w.balance
}

// Balance returns the amount of the assets in this wallet in units of AVAX
func (w *Wallet) BalanceForDisplay(assetID ids.ID) string {
	if w.IsFetching() {
		return "<fetching balance>"
	} else {
		f := float64(w.Balance(assetID)) / float64(units.Avax)
		return fmt.Sprintf("%f", f)
	}
}

// CreateTx returns a tx that sends [amount] of [assetID] to [destAddr]
// TODO this only works for avax, make it work for any asset id
func (w *Wallet) CreateTx(assetID ids.ID, amount uint64, destAddr ids.ShortID, memo string) (*avm.Tx, error) {
	displayAddr, err := formatting.FormatAddress(w.Config.ChainName, w.Config.NetworkName, destAddr.Bytes())
	if err != nil {
		return nil, errors.New("error converting destaddr to Avax format")
	}

	log.Printf("CreateTx: assetID: %v, amount(uint64): %d, destAddr: %v, memo: %s", w.FormatAssetID(assetID), amount, displayAddr, memo)

	bal := w.Balance(assetID)

	if amount == 0 {
		return nil, errors.New("invalid amount 0")
	}

	if amount > bal {
		return nil, errors.New("insufficient balance")
	}

	amountSpent := uint64(0)
	time := w.clock.Unix()

	ins := []*avax.TransferableInput{}
	for _, utxo := range w.utxoSet.Sorted() {
		if utxo.AssetID() != assetID {
			continue
		}
		inputIntf, err := w.keychain.Spend(utxo.Out, time)
		if err != nil {
			continue
		}
		input, ok := inputIntf.(avax.TransferableIn)
		if !ok {
			continue
		}
		spent, err := math.Add64(amountSpent, input.Amount())
		if err != nil {
			return nil, err
		}
		amountSpent = spent

		in := &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: assetID},
			In:     input,
		}

		ins = append(ins, in)

		if amountSpent >= amount {
			break
		}
	}

	if amountSpent < amount {
		return nil, errors.New("insufficient funds")
	}

	outs := []*avax.TransferableOutput{{
		Asset: avax.Asset{ID: assetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: amount,
			OutputOwners: secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{destAddr},
			},
		},
	}}

	amountWithFee, err := math.Add64(amount, txFee)
	if err != nil {
		return nil, fmt.Errorf("problem calculating required spend amount: %w", err)
	}

	if amountSpent > amountWithFee {
		// TODO HD wallet addresses?
		changeAddr := w.PublicKeyAvm().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: assetID},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - amountWithFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	avax.SortTransferableInputs(ins)
	avax.SortTransferableOutputs(outs, w.codec)

	// TODO support different types of memo like avalanchejs does?
	memoBytes := []byte(memo)

	tx := &avm.Tx{UnsignedTx: &avm.BaseTx{BaseTx: avax.BaseTx{
		NetworkID:    w.Config.NetworkID,
		BlockchainID: w.Config.ChainID,
		Outs:         outs,
		Ins:          ins,
		Memo:         memoBytes,
	}}}

	return tx, nil
}

func (w *Wallet) GetUnsignedBytes(source interface{}) ([]byte, error) {
	c := w.Codec()
	unsignedBytes, err := c.Marshal(codecVersion, source)
	return unsignedBytes, err
}

// Convert the signature generated by the MPS protocol into an Avalance recoverable signature
func (w *Wallet) MpsSigToAvaSig(hashedmsg []byte, mpssig *mpsecdsa.Signature) ([]byte, error) {
	// cborbytes, err := cbor.Marshal(mpssig)
	// log.Printf("mpssig cbor bytes: %v", cborbytes)

	rb, err := mpssig.R.XScalar().MarshalBinary()
	if err != nil {
		return []byte{}, err
	}

	sb, err := mpssig.S.MarshalBinary()
	if err != nil {
		return []byte{}, err
	}

	// Avalanche sig is r | s | v  where v is the recovery byte
	var sigava [65]byte
	copy(sigava[0:32], rb[0:32])
	copy(sigava[32:64], sb[0:32])
	log.Printf("mpsecdsa.Signature r | s: %v", sigava)

	// Check if we have to negate the s value, in order to pass the verifySECP256K1RSignatureFormat check in the avalanche code
	// https://github.com/ava-labs/avalanchego/blob/cf80745d32fb61504ccccb689c5ed84c09a28a73/utils/crypto/secp256k1r.go#L217
	var s secp256k1.ModNScalar
	s.SetByteSlice(sigava[32:64])
	if s.IsOverHalfOrder() {
		s.Negate()
		sneg := s.Bytes()
		copy(sigava[32:64], sneg[:])
	}

	// Try all the recovery codes and see which one is the correct one
	// TODO SECURITY There must be a better way to do this but brute-forcing it for now
	codes := []byte{0, 1, 2, 3, 4}
	for _, c := range codes {
		sigava[64] = c
		f := avacrypto.FactorySECP256K1R{}
		pubkeyRecovered, err := f.RecoverHashPublicKey(hashedmsg, sigava[:])
		if err == nil {
			pubkeyRecoveredAva := pubkeyRecovered.(*avacrypto.PublicKeySECP256K1R)
			if pubkeyRecoveredAva != nil {
				a1 := pubkeyRecoveredAva.Address()
				a2 := w.PublicKeyAvm().Address()
				if a1 == a2 {
					return sigava[:], nil
				}
			}
		} else {
			log.Printf("MpsSigToAvaSig err with Code: %v  Err: %v", c, err)
		}
	}

	return []byte{}, errors.New("recovered public key didnt match original public key")
}

func (w *Wallet) VerifyHash(hashedmsg []byte, mpssig *mpsecdsa.Signature) bool {
	_, err := w.MpsSigToAvaSig(hashedmsg, mpssig)
	return err == nil
}

func (w *Wallet) FormatTxURL(txID ids.ID) string {
	return fmt.Sprintf(w.Config.ExplorerURL, txID.String())
}

// Return asset info
func (w *Wallet) FormatAssetID(assetID ids.ID) string {
	switch assetID.String() {
	case w.Config.AssetID.String():
		return w.Config.AssetName
	default:
		return assetID.String()
	}
}

func (w *Wallet) GetAsset(assetID ids.ID) Asset {
	return w.assets[assetID]
}

// Dump all UTXOs to a formatted string for inspection
func (w *Wallet) DumpUTXOs() string {
	// return w.utxoSet.String()
	s := ""
	for _, utxo := range w.utxoSet.Sorted() {
		out, ok := utxo.Out.(*secp256k1fx.TransferOutput)
		if ok {
			asset := w.GetAsset(utxo.AssetID())
			amt := w.FormatAmount(asset, out.Amt)
			s += fmt.Sprintf("    Asset: %s Amt: %v TxID: %s Idx: %v \n", asset.Symbol, amt, utxo.TxID, utxo.OutputIndex)
		}
	}
	return s
}

func (w *Wallet) FormatAmount(asset Asset, amt uint64) string {
	return fmt.Sprintf("%.4f", float64(amt)/stdmath.Pow(10, float64(asset.Denomination)))
}

func (w *Wallet) FormatIssueTxAsCurl(tx string) string {
	template := `
curl -X POST '%s/ext/bc/X' \
--header 'Content-Type: application/json' \
--data-raw '{
    "jsonrpc":"2.0",
    "id"     : 1,
    "method" :"avm.issueTx",
    "params" :{
        "tx":"%s"
    }
}'	
	`
	return fmt.Sprintf(template, w.Config.RPCHostURL, tx)
}
