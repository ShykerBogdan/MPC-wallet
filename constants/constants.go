package constants

import (
	"github.com/ava-labs/avalanchego/ids"
)

// TODO Figure out how to abstract all this for supporting P-chain and multiple UTXO-based blockchains BTC/LTC etc

type ChainConfig struct {
	Blockchain string
	NetworkName string
	NetworkID uint32
	ChainName string
	ChainID ids.ID
	AssetName string
	AssetID ids.ID
	RPCHostURL string
	ExplorerURL string
}

var fujiChainID, _ = ids.FromString("2JVSBoinj9C2J33VntvzYtVJNZdN2NKiwwKjcumHUWEb5DbBrm") // Fuji X-chain
var fujiAssetID, _ = ids.FromString("U8iRqJoiJm8xZHAacmvYyZVwqQx6uDNtQeP3CQ6fcgQk3JqnK") // Fuji AVAX

var AvmFujiConfig = ChainConfig{
	Blockchain:  "avalanche",
	NetworkName: "fuji",
	NetworkID:   5,
	ChainName:   "X",
	ChainID:     fujiChainID,
	AssetName:   "avax",
	AssetID:     fujiAssetID,
	RPCHostURL:  "https://api.avax-test.network:443",
	//RPCHostURL:  "https://rpc.ankr.com/avalanche_fuji",
	//RPCHostURL:  "https://api.avax-test.network/ext/bc/C/rpc",
	ExplorerURL: "https://explorer.avax-test.network/tx/%s",
}

var mainnetChainID, _ = ids.FromString("2oYMBNV4eNHyqk2fjjV5nVQLDbtmNJzq5s3qs3Lo6ftnC6FByM") // mainnet X-chain
var mainnetAssetID, _ = ids.FromString("FvwEAhmxKfeiG8SnEvq42hc6whRyY3EFYAvebMqDNDGCgxN5Z") // mainnet AVAX

var AvmMainnetConfig = ChainConfig{
	Blockchain:  "avalanche",
	NetworkName: "mainnet",
	NetworkID:   1,
	ChainName:   "X",
	ChainID:     mainnetChainID,
	AssetName:   "avax",
	AssetID:     mainnetAssetID,
	RPCHostURL:  "https://api.avax.network:443",
	ExplorerURL: "https://explorer.avax.network/tx/%s",
}

var goerliChainID, _ = ids.FromString("2oYMBNV4eNHyqk2fjjV5nVQLDbtmNJzq5s3qs3Lo6ftnC6FByM")
var goerliAssetID, _ = ids.FromString("0xFfb99f4A02712C909d8F7cC44e67C87Ea1E71E83") 

var GoerliConfig = ChainConfig{
	Blockchain:  "Ethereum",
	NetworkName: "mainnet",
	NetworkID:   5,
	ChainName:   "Goerli",
	ChainID:     goerliChainID,
	AssetName:   "eth",
	AssetID:     mainnetAssetID,
	RPCHostURL:  "https://goerli.blockpi.network/v1/rpc/public",
	ExplorerURL: "https://goerli.etherscan.io/tx/%s",
}


