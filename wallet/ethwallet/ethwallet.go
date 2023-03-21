package ethwallet

import (
	stdecdsa "crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math"
	stdmath "math"
	"math/big"
	"sort"
	"sync"
	"time"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v3"
	
    secp256k1Eth "github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shykerbogdan/mpc-wallet/constants"
	"github.com/shykerbogdan/mpc-wallet/user"

	"github.com/fxamacker/cbor/v2"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/conn"
	ethcrypto "github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/crypto"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/types"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	mpsconfig "github.com/taurusgroup/multi-party-sig/protocols/cmp/config"
)

type Asset struct {
	AssetID      string
	Name         string
	Symbol       string
	Denomination uint8
}

type Wallet struct {
	Name      string
	Threshold int
	Me        user.User
	Others    []user.User
	KeyData   []byte
	// Public address computed from the MPC config and stored here so it shows up in the persisted JSON for reference
	Address string
	// Config params for a blockchain
	Config constants.ChainConfig

	CreatedAt time.Time

	// For now we use just one pub/priv key, maybe use BIP32 to enhance privacy?
	//keychain *Keychain

	balance big.Int

	// Set of unique asset ids contained in the utxos
	assets     map[string]Asset

	conn       *conn.EthConn

	erc20List  []*types.Erc20Token
	Key        *ethcrypto.Key

	isFetching bool
	mutex      sync.Mutex
}

// NewWallet returns a new Goerli Wallet
func NewEmptyWallet(network string, name string, threshold int, me user.User, others []user.User) *Wallet {
	w := &Wallet{
		Name:      name,
		Threshold: threshold,
		Me:        me,
		Others:    others,
		CreatedAt: time.Now().UTC(),

		//keychain:  NewKeychain(),
		balance: *big.NewInt(int64(0)), //map[string]uint64{},
		assets:  map[string]Asset{},
	}

	switch network {
	case "goerli":
		w.Config = constants.GoerliConfig
		// case "fuji":
		// 	w.Config = constants.AvmFujiConfig
		// case "mainnet":
		// 	w.Config = constants.AvmMainnetConfig
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
	w.balance = *big.NewInt(0)
	w.assets = map[string]Asset{}
	w.KeyData = keydata
	//w.keychain = NewKeychain()
	//w.Key = NewKeyFromECDSA(w.PublicKeyEth())

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

// Unmarshal the MPC config which contains the key data
func (w *Wallet) GetUnwrappedKeyData() mpsconfig.Config {
	c := mpsconfig.EmptyConfig(curve.Secp256k1{})
	err := cbor.Unmarshal(w.KeyData, c)
	if err != nil {
		log.Fatalf("GetUnwrappedKeyData error %v", err)
	} 
	return *c
}

// From the MPC key data, convert to an eth public key
func (w *Wallet) PublicKeyEth() stdecdsa.PublicKey {
	kd := w.GetUnwrappedKeyData()
	ppb, err := kd.PublicPoint().MarshalBinary()
	if err != nil {
		panic(err)
	} 

	key, err := secp256k1.ParsePubKey(ppb)
	ecdsakey := key.ToECDSA()

	if err != nil {
		panic(err)
	} 

	return *ecdsakey
}

func (w *Wallet) PublicKeyMpsPoint() curve.Point {
	kd := w.GetUnwrappedKeyData()
	ppb := kd.PublicPoint()
	return ppb
}

func (w *Wallet) GetFormattedAddress() string {
	b := w.PublicKeyEth()
	return ethcrypto.PubkeyToAddress(b).String()
}

func (w *Wallet) GetCommonAddress() common.Address {
	b := w.PublicKeyEth()
	return ethcrypto.PubkeyToAddress(b)
}

func (w *Wallet) clearBalances() {
	w.balance = *big.NewInt(0)
}

func (w *Wallet) IsFetching() bool {
	return w.isFetching
}

// Run in a Go routine, as well as called directly
func (ew *Wallet) FetchBalance() error {
	ew.mutex.Lock()
	ew.isFetching = true
	defer ew.doneFetching()
	defer ew.mutex.Unlock()	

	balance, err := ew.conn.GetBalance(ew.GetCommonAddress())
	ew.balance = *balance

	if err != nil {
		return err
	}
	return nil
}

func (w *Wallet) doneFetching() {
	w.isFetching = false
}

// Balance returns the amount of the assets in this wallet
func (w *Wallet) Balance() uint64 {
	return w.balance.Uint64()
}

func (w *Wallet) BalanceForDisplay(assetID string) string {
	if w.IsFetching() {
		return "<fetching balance>"
	} else {
		f := float64(w.Balance())
		return fmt.Sprintf("%f", f)
	}
}

func (ew *Wallet) SendRawTransaction(raw string) (string, error) {
	txid, err := ew.conn.SendRawTransaction(raw)
	if err != nil {
		return "", fmt.Errorf("SendRawTransaction occured error: %s\n", err)
	}
	return txid, nil
}

func (ew *Wallet) PublishTx(rawTx string) (txid string, err error) {
	txid, err = ew.SendRawTransaction(rawTx)
	if err != nil {
		return "", fmt.Errorf("SendRawTransaction occured error:%s \n", err)
	}
	return
}

func (ew *Wallet) CreateNormalTransaction(to *common.Address, value *big.Int, data []byte, gasPrice *big.Int, gasLimit uint64) (*types.Transaction, error) {
	var tx *types.Transaction
	var err error
	if gasPrice == nil || gasPrice.Cmp(big.NewInt(0)) == 0 {
		gasPrice, err = ew.GetGasPrice()
		if err != nil {
			return nil, fmt.Errorf("GetGasPrice occured error:%s \n", err)
		}
	}
	nonce, err := ew.GetNonce(types.Latest)
	if err != nil {
		return nil, fmt.Errorf("GetNonce occured error:%s \n", err)
	}

	address := ew.GetCommonAddress()

	tx = &types.Transaction{
		From:     &address,
		To:       to,
		GasPrice: gasPrice,
		Nonce:    nonce,
		Value:    value,
		Data:     data,
	}
	if gasLimit == 0 {
		txr := tx.ToTransactionRequest()
		gasLimit, err = ew.GetGasLimit(txr)
		if err != nil {
			return nil, fmt.Errorf("GetGasLimit occured error:%v \n", err)
		}
	}
	tx.GasLimit = gasLimit
	return tx, nil
}

// Convert the signature generated by the MPC protocol into an eth recoverable signature
func (w *Wallet) MpcSigToEthSig(hashedmsg []byte, mpcsig *mpsecdsa.Signature) ([]byte, error) {
	rb, err := mpcsig.R.XScalar().MarshalBinary()
	if err != nil {
		return []byte{}, err
	}

	sb, err := mpcsig.S.MarshalBinary()
	if err != nil {
		return []byte{}, err
	}

	rs := append(rb, sb...)
	v, err := secp256k1Eth.RecoverPubkey(hashedmsg, rs)

	 
    // Convert V to an Ethereum-compatible format
    if v[31] == 0 {
        v[31] = 27
    } else if v[31] == 1 {
        v[31] = 28
    } else {
        v[31] += 4
    }
    
    // Concatenate R, S, and V into a single slice in Ethereum format
    ethSig := append(rs, v[31])

	return ethSig, nil

	// // Eth sig is r | s | v  where v is used to encode the recovery ID
	// var sigeth [65]byte
	// copy(sigeth[0:32], rb[0:32])
	// copy(sigeth[32:64], sb[0:32])
	// copy(sigeth[64:], []byte{sigeth[64] + 35 + w.Config.NetworkID*2})
	// return sigeth[:], nil
}

func CheckValueEnough(value *big.Int, gasPrice *big.Int, gasLimit uint64, ether *big.Int) bool {
	tvalue := big.NewInt(0).Set(value)
	tgasPrice := big.NewInt(0).Set(gasPrice)
	if tvalue.Add(tvalue, tgasPrice.Mul(tgasPrice, big.NewInt(int64(gasLimit)))).Cmp(ether) == 1 {
		return false
	}
	return true
}

func (w *Wallet) VerifyHash(hashedmsg []byte, mpssig *mpsecdsa.Signature) bool {
	_, err := w.MpcSigToEthSig(hashedmsg, mpssig)
	return err == nil
}

func (w *Wallet) FormatTxURL(txID string) string {
	return fmt.Sprintf(w.Config.ExplorerURL, txID)
}

func (w *Wallet) GetAsset(assetID string) Asset {
	return w.assets[assetID]
}

func (w *Wallet) FormatAmount(asset Asset, amt uint64) string {
	return fmt.Sprintf("%.4f", float64(amt)/stdmath.Pow(10, float64(asset.Denomination)))
}

func (ew *Wallet) GetBalance() (*big.Int, error) {
	balance, err := ew.conn.GetBalance(ew.GetCommonAddress())
	if err != nil {
		return big.NewInt(0), err
	}
	return balance, nil
}

func (ew *Wallet) GetGasPrice() (*big.Int, error) {
	gasPrice, err := ew.conn.GetGasPrice()
	if err != nil {
		return big.NewInt(0), err
	}
	return gasPrice, nil
}

func (ew *Wallet) GetNonce(param types.BlockParam) (uint64, error) {
	nonce, err := ew.conn.GetNonce(ew.GetCommonAddress())
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (ew *Wallet) GetGasLimit(tx *types.TransactionRequest) (uint64, error) {
	gaslimit, err := ew.conn.GetEstimateGas(*tx)
	if err != nil {
		return 0, err
	}
	return gaslimit, nil
}

func (ew *Wallet) ConstructEtherscanUrl(networkName, txid string) string {
	if networkName == types.EthereumNet.Name {
		return fmt.Sprintf("https://etherscan.io/tx/%s\n", txid)
	} else {
		return fmt.Sprintf("https://%s.etherscan.io/tx/%s\n", networkName, txid)
	}
}

func (ew *Wallet) ToCommonAddress(address string) common.Address {
	return common.HexToAddress(address)
}

func weiToEther(wei *big.Int) string {
	fwei := new(big.Float)
	fwei.SetString(wei.String())
	ethValue := new(big.Float).Quo(fwei, big.NewFloat(math.Pow10(18)))
	return ethValue.String()
}
