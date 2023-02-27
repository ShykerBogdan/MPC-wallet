package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/johnthethird/thresher/user"
	"github.com/johnthethird/thresher/wallet/avmwallet"
)

type AppConfig struct {
	// avalanche
	Blockchain string

	// mainnet, fuji
	Network string

	// Name of the project, e.g. DAO-SuperSwap
	Project string

	// "chatnet" uses libp2p. Could also implement Keybase chat? Others?
	P2PNetwork string

	Me user.Me

	// TODO Implement wallet types / factory for other chains (BTC, etc)
	Wallets map[string]*avmwallet.Wallet 

	UpdatedAt time.Time

	filename string
	isLoaded bool
	mutex sync.Mutex
}

var errUnsupportedBlockchain = errors.New("Blockchain/Network is unsupported")

// Create a new AppConfig
func New(blockchain string, network string, project string, nick string, address string) (*AppConfig, error) {
	if (blockchain != "avalanche") || (network != "mainnet" && network != "fuji") {
		return nil, errUnsupportedBlockchain
	}

	me, err := user.NewMe(nick, address)
	if err != nil {
		return nil, err
	}

	cfg := &AppConfig{
		Blockchain: blockchain, 
		Network: network, 
		Me: me, 
		Project: project,
		P2PNetwork: "chatnet", 
		Wallets: make(map[string]*avmwallet.Wallet), 
		isLoaded: false,
	}

	return cfg, nil
}


// Initialize the AppConfig from a marshalled file on disk
func Load(filename string) *AppConfig {
	if !FileExists(filename) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s \nRun 'thresher help init' for more info.", filename)
		os.Exit(1)
	}

	jsonb, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error reading config file %s: %v \n", filename, err)
		os.Exit(1)
	}		

	ac := &AppConfig{}
	err = json.Unmarshal(jsonb, ac)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error loading config file %s: %v \n", filename, err)
		os.Exit(1)
	}		

	ac.filename = filename
	ac.isLoaded = true

	return ac
}

// Name of the config file on disk
func (ac *AppConfig) CfgFile() string {
	return ac.filename
}

// Has the file been loaded from disk
func (ac *AppConfig) IsLoaded() bool {
	return ac.isLoaded
}

// Save config file to disk as filename
func (ac *AppConfig) Save(filename string) error {
	if FileExists(filename) {
		return fmt.Errorf("config file %s already exists", filename)
	}
	ac.filename = filename
	ac.Persist()
	return nil
}

// Write the current config back to disk
func (ac *AppConfig) Persist() {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	now := time.Now().UTC()

	// TODO should probably do something like this and keep some backups?
	// if ac.Exists() {
	// 	backupname := ac.cfgFile + now.Format(".2006-01-02T15-04-05.0") + ".bak"

	// 	err := os.Rename(ac.cfgFile, backupname)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "Fatal error saving backup config file %s: %v \n", backupname, err)
	// 		os.Exit(1)
	// 	}
	// }

	ac.UpdatedAt = now

	jsonb, err := json.MarshalIndent(ac, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error preparing to save config file %s: %v \n", ac.filename, err)
		os.Exit(1)
	}

	err = write(ac.filename, jsonb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error saving config file %s: %v \n", ac.filename, err)
		os.Exit(1)
	}
	ac.isLoaded = true
}

// Create a new empty wallet which will hold a multisig key after the multi-party keygen protocol has been completed
func (ac *AppConfig) NewEmptyWallet(name string, threshold int, signers []user.User) *avmwallet.Wallet {
	others := []user.User{}
	for _, u := range signers {
		if u.Address != ac.Me.Address {
			others = append(others, u)
		}
	}

	w := avmwallet.NewEmptyWallet(ac.Network, name, threshold, ac.Me.User, others)
	return w
}

func (ac *AppConfig) FindWallet(name string) *avmwallet.Wallet {
	return ac.Wallets[name]
}

func (ac *AppConfig) AddWallet(w *avmwallet.Wallet) error {
	ac.mutex.Lock()
	ac.Wallets[w.GetName()] = w
	ac.mutex.Unlock()

	ac.Persist()
	return nil
}

func (ac *AppConfig) RenameWallet(oldName string, newName string) error {
	ac.mutex.Lock()
	
	_, found := ac.Wallets[newName]
	if found {
		return errors.New("Cannot rename wallet, new name already exists")
	}

	if w, ok := ac.Wallets[oldName]; ok {
		w.SetName(newName)
		ac.Wallets[newName] = w
		delete(ac.Wallets, oldName)
	}

	ac.mutex.Unlock()

	ac.Persist()

	return nil
}

func (ac *AppConfig) SortedWalletNames() []string {
	arr := []string{}
	for _, v := range ac.Wallets { 
   arr = append(arr, v.GetName())
	}

	sort.Slice(arr, func(i, j int) bool {
		return arr[i] < arr[j]
	})

	return arr
}

// TODO this is not used currently, also it would reset libp2p keys. Is that OK?
func (ac *AppConfig) SetMe(nick string, address string, signednick string) error {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	u, err := user.NewMe(nick, signednick)
	if err != nil {
		return err
	}

	ac.Me = u
	return nil
}

func (ac *AppConfig) String() string {
	msg := fmt.Sprintf(`
  Config File: %s			
  Blockchain: %s
  Network: %s
  Project: %s
  Nick: %s
  PeerID: %s
  Address: %s
			`, ac.CfgFile(), ac.Blockchain, ac.Network, ac.Project, ac.Me.Nick, ac.Me.PeerID(), ac.Me.Address)
	return msg
}

func (ac *AppConfig) Exists() bool {
	return FileExists(ac.filename)
}

func (ac *AppConfig) MustExist() {
	if !FileExists(ac.filename) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\nRun 'thresher help init' for more info.", ac.filename)
		os.Exit(1)
	}
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Create and write a new file
func write(name string, data []byte) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}

	return f.Sync()
}
