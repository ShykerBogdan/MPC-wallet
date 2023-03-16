package ethwallet

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/conn"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/types"

	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/shykerbogdan/mpc-wallet/constants"
	"github.com/shykerbogdan/mpc-wallet/user"
	"github.com/shykerbogdan/mpc-wallet/wallet/avmwallet"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
)

type Wallet struct {
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

	keychain *avmwallet.Keychain

	balance map[string]uint64
	// Set of unique asset ids contained in the utxos
	assets map[string]avmwallet.Asset

	// txs []*avm.Tx

	isFetching bool
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
		keychain:  avmwallet.NewKeychain(),
		balance:   map[string]uint64{},
		assets:    map[string]avmwallet.Asset{},
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

type EthereumWallet struct {
	conn      *conn.EthConn
	Wallet    *Wallet
	erc20List []*types.Erc20Token
}

func NewEthereumWallet(auth, path string, config types.Config) (*EthereumWallet, error) {
	wallet, err := CreateNewWallet(auth, path, config)
	if err != nil {
		return nil, err
	}
	return &EthereumWallet{
		conn:      conn.NewEthConn(config.ServerUrl),
		Wallet:    wallet,
		erc20List: config.Erc20List,
	}, nil
}

func (ew *EthereumWallet) GetBalance() (*big.Int, error) {
	balance, err := ew.conn.GetBalance(ew.Wallet.Key.Address)
	if err != nil {
		return big.NewInt(0), err
	}
	return balance, nil
}

func (ew *EthereumWallet) GetGasPrice() (*big.Int, error) {
	gasPrice, err := ew.conn.GetGasPrice()
	if err != nil {
		return big.NewInt(0), err
	}
	return gasPrice, nil
}

func (ew *EthereumWallet) GetNonce(param types.BlockParam) (uint64, error) {
	nonce, err := ew.conn.GetNonce(ew.Wallet.Key.Address)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (ew *EthereumWallet) GetGasLimit(tx *types.TransactionRequest) (uint64, error) {
	gaslimit, err := ew.conn.GetEstimateGas(*tx)
	if err != nil {
		return 0, err
	}
	return gaslimit, nil
}

func (ew *EthereumWallet) GetErc20ListBalance() (map[string]*big.Int, error) {
	list, err := ew.conn.GetErc20Balance(ew.Wallet.Key.Address)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (ew *EthereumWallet) SendRawTransaction(raw string) (string, error) {
	txid, err := ew.conn.SendRawTransaction(raw)
	if err != nil {
		return "", fmt.Errorf("SendRawTransaction occured error: %s\n", err)
	}
	return txid, nil
}

func (ew *EthereumWallet) createNormalTransaction(to *common.Address, value *big.Int, data []byte, gasPrice *big.Int, gasLimit uint64) (*types.Transaction, error) {
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
	tx = &types.Transaction{
		From:     &ew.Wallet.Key.Address,
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

func (ew *EthereumWallet) createErc20Transation(token *types.Erc20Token, value *big.Int, to *common.Address, gasPrice *big.Int, gasLimit uint64) (*types.Transaction, error) {
	data := token.GenerateTransferData(value, to)
	tx, err := ew.createNormalTransaction(token.Address, big.NewInt(0), data, gasPrice, gasLimit)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (ew *EthereumWallet) TransferEther(to *common.Address, value *big.Int, data []byte, gasPrice *big.Int, gasLimit uint64) (txid string, err error) {
	ether, err := ew.GetBalance()
	if err != nil {
		return "", fmt.Errorf("get balance occured error: %s\n", err)
	}
	// tx, err := ew.createNormalTransaction(to, value, []byte{}, big.NewInt(0), 0)
	tx, err := ew.createNormalTransaction(to, value, data, gasPrice, gasLimit)
	if err != nil {
		return "", fmt.Errorf("createNormalTransaction occured error: %s\n", err)
	}
	ok := checkValueEnough(tx.Value, tx.GasPrice, tx.GasLimit, ether)
	if !ok {
		return "", fmt.Errorf("your transaction's cost is bigger then ethers you own")
	}
	txid, err = ew.signAndPublishTx(tx)
	if err != nil {
		return "", fmt.Errorf("signAndPublishTx occured error:%s \n", err)
	}
	return
}

func checkValueEnough(value *big.Int, gasPrice *big.Int, gasLimit uint64, ether *big.Int) bool {
	tvalue := big.NewInt(0).Set(value)
	tgasPrice := big.NewInt(0).Set(gasPrice)
	if tvalue.Add(tvalue, tgasPrice.Mul(tgasPrice, big.NewInt(int64(gasLimit)))).Cmp(ether) == 1 {
		return false
	}
	return true
}

func (ew *EthereumWallet) TransferErc20(token *types.Erc20Token, value *big.Int, to *common.Address, gasPrice *big.Int, gasLimit uint64) (txid string, err error) {
	tx, err := ew.createErc20Transation(token, value, to, gasPrice, gasLimit)
	//tx, err := ew.createErc20Transation(token, value, to, big.NewInt(0),0)
	if err != nil {
		return "", err
	}
	txid, err = ew.signAndPublishTx(tx)
	if err != nil {
		return "", err
	}
	return
}

func (ew *EthereumWallet) signAndPublishTx(tx *types.Transaction) (txid string, err error) {
	rawTx, err := ew.Wallet.SignTxToRawTx(tx)
	if err != nil {
		return "", fmt.Errorf("SignTxToRawTx occured error:%s \n", err)
	}
	txid, err = ew.SendRawTransaction(rawTx)
	if err != nil {
		return "", fmt.Errorf("SendRawTransaction occured error:%s \n", err)
	}
	return
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
