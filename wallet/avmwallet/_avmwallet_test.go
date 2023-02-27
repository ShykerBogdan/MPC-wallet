package avmwallet

import (
	"encoding/base64"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v3"
	"github.com/fxamacker/cbor/v2"
	"github.com/johnthethird/thresher/user"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	"github.com/stretchr/testify/require"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
)

var users = map[string]user.User{}
var keydata []byte
var chatroom = "chatroom"

func init() {
	factory := crypto.FactorySECP256K1R{}

	var keys  []*crypto.PrivateKeySECP256K1R
	var addrs []ids.ShortID // addrs[i] corresponds to keys[i]

	for _, key := range []string{
		"24jUJ9vZexUM6expyMcT48LBx27k1m7xpraoV62oSQAHdziao5",
		"2MMvUMsxx6zsHSNXJdFD8yc5XkancvwyKPwpw4xUK3TCGDuNBY",
		"cxb7KpGWhDMALTjNNSJ7UQkkomPesyWAPUaWRGdyeBNzR6f35",
	} {
		keyBytes, _ := formatting.Decode(formatting.CB58, key)
		pk, _ := factory.ToPrivateKey(keyBytes)
		keys = append(keys, pk.(*crypto.PrivateKeySECP256K1R))
		addrs = append(addrs, pk.PublicKey().Address())
	}

	for i, nick := range []string{
		"alice",
		"bob",
		"cam",
	} {
		users[nick] = user.NewUserFromPrivateKey("X", "fuji", chatroom, nick, keys[i])
	}

	// pub key X-fuji10xqhzdxvc4r2v7za5tq5xra3c088kyhe8c8lsd
	keydata64 := "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"
	b, err := base64.StdEncoding.DecodeString(keydata64)
	if err != nil {
		panic("unable to decode keydata")
	}
	keydata = b
}

func AddUTXO(w *Wallet, utxo *avax.UTXO) {
	out, ok := utxo.Out.(avax.TransferableOut)
	if !ok {
		return
	}

	w.utxoSet.Put(utxo)
	w.balance[utxo.AssetID()] += out.Amount()
}

func TestNewWallet(t *testing.T) {
	w := NewEmptyWallet("fuji", "treasury", 1, users["alice"], []user.User{users["bob"], users["cam"]})
	if w == nil {
		t.Fatalf("failed to create the new wallet")
	}
}

func TestMutatedSig2(t *testing.T) {
	w := NewEmptyWallet("fuji", "treasury", 1, users["alice"], []user.User{users["bob"], users["cam"]})
	w.Initialize(keydata)

	// GOOD
	cborbytes := []byte{162,97,82,88,33,3,113,70,96,15,193,84,241,151,148,12,197,190,142,153,166,128,226,19,133,93,193,202,241,221,148,223,18,252,200,241,207,194,97,83,88,32,76,125,82,1,117,143,216,224,184,177,139,199,88,95,204,59,188,144,252,158,84,38,218,216,12,14,219,90,163,195,7,232}
	msghash, err := ids.FromString("CkkMc9EYD2RS2sqQ6Upk3QgfRBjJDrDDipwbsvbf438yJ38ZT")

	// BAD
	// cborbytes := []byte{162,97,82,88,33,3,255,119,71,205,109,247,23,161,204,196,121,224,180,7,28,197,108,255,7,83,38,69,116,156,122,161,30,60,34,21,68,115,97,83,88,32,215,203,0,84,182,201,232,125,234,187,143,22,82,56,145,243,177,146,93,239,52,132,64,245,62,195,250,185,118,115,124,15}
	// msghash, err := ids.FromString("4Ec45RbuFKNSF3X4fxjButzQDa1RpCEMe46pAzzExY9Y59J9n")

	require.NoError(t, err)

	mpssig := mpsecdsa.EmptySignature(curve.Secp256k1{})
	if err = cbor.Unmarshal(cborbytes, &mpssig); err != nil {
		t.Fatalf("OUCH %v", err)
	}

	if !mpssig.Verify(w.PublicKeyMpsPoint(), msghash[:]) {
		t.Log("Invalid sig")
	}

	// Now convert to Ava
	rb, err := mpssig.R.XScalar().MarshalBinary()
	if err != nil {t.Fatal("R")}

	sb, err := mpssig.S.MarshalBinary()
	if err != nil {t.Fatal("S")}

	var sigava [65]byte
	copy(sigava[0:32], rb[0:32])
	copy(sigava[32:64], sb[0:32])
	t.Logf("mpsecdsa.Signature (RSV): %v", sigava)

	var s secp256k1.ModNScalar
	z := s.SetByteSlice(sigava[32:64])
	t.Logf("overflow? %v Odd?: %v", z, s.IsOdd())
	if s.IsOverHalfOrder() {
		t.Log("Mutated, negating")
		s.Negate()
		news := s.Bytes()
		copy(sigava[32:64], news[:])
	}

	// Try all the recovery codes and see which one is the correct one
	codes := []byte{0, 1, 2, 3, 4}
	for _, c := range codes {
		sigava[64] = c
		t.Logf("Sigava: %v", sigava[:])
		f := crypto.FactorySECP256K1R{}
		pubkeyRecovered, err := f.RecoverHashPublicKey(msghash[:], sigava[:])
		if err == nil {
			pubkeyRecoveredAva := pubkeyRecovered.(*crypto.PublicKeySECP256K1R)
			if pubkeyRecoveredAva != nil {
				a1 := pubkeyRecoveredAva.Address()
				a2 := w.PublicKeyAvm().Address()
				if a1 == a2 {
					t.Logf("MATCHED Sigava: %v", sigava[:])
					break
				}
			}
		} else {
			t.Logf("MpsSigToAvaSig err with Code: %v  Err: %v", c, err)
		}
	}



	t.Fatal()
}	

func TestMutatedSig1( t *testing.T) {
	w := NewEmptyWallet("fuji", "treasury", 1, users["alice"], []user.User{users["bob"], users["cam"]})
	w.Initialize(keydata)

	// This one had error signature was mutated from its original format
	//txcb58 := "1111111114QVHAgReanECBWVHeabjsijZaBnMuidXUtHQnJ1MU8rKL2HQFfLBoc3BWYyNKFjNZiHLYf6cZuCkiiymGsraX9PMxQA23Tu2yrWyp7GPQ4RjUAMDZwhhksTKNiGings8226QFyP5wiisePa5qhknkigCNTcE34ngFjiGUEEGYbZaePR9uLGr2WyhoEM5Nw9wS2H8kz45CrFaA6FBpRTFmCCh84gCU71CZGddRe3a62cCbUUWhh22KUZbYXZTKpe363Ltod2i8z44yjW812QS4A1YcTdQw8QeLszuv8ZkpkLSgQQxaW6FqgVQBhqrQ5xGsnEFBbxZMjVhbTidLYvzGMsizGPcJWKj8s2r4g93u2gxMMhjHiNSsLSsCSM1EkNNaGbHtcCEYYoPHQQiXZ7hxjUWHjj8pTTmRXWoQUrvaiimJYK3JDLQXGupgTaxhFUa1MFD9SiBvTJA2fGe532eqVwHX6s54CcvsMp9QqZX3AxGZk43S3GuzaSZunixGumXi54YNSYqhL3Pmkn3Z8oNRyCitWz8rsCGz76QT3juTyowgvmdSPcwbuAyE6tgvmfAvPLNjYyyLn8oFjidxfK9Cm89S8CoDzjxEy43rPXhtV6phkckNubjQgRLLMcHWGxHcAzjoDpBJSfhSwzNzQMAib6fpUK2vj9o5JpRcsz7U953FuSQpTw46dwdYoR2vSnQrwEDMr3QxpD4C863fnaT9mTW4oFpYPNFKaR2M4ptZeP5z6yqb7P2DHS1Ft9c2YSSiEPeHnNhpSWhRpiqHPiaRPzyJpaWBUUTZpeWEramh45GygQ9PRXj5FG61mwucx2vnkj5Lmt2c99iQWnoFXbbXxuU5ehn94MXm9EEVyvwSFWtvsCfftQTzKGayYGzZNYpjxGnX1xwAAKMg3hdCTdczW5SquxD1UKe6pGoGBQewJnHcUh4nyfKrQhotaHEZT2uonqGguuo9dgGAHWEJjL7E1SaP2Gv24uRFoy4GTzmQdxRudJ7oKJfNGJLid1ddrkAEQUYo52o5eQfqo6i4rLVRaxRNk2hGgpaQgkR3TLPqBto36J5R5uJ5hCsirp3tEMaG9seeNYRnkh41SVc8WS4FAsCy2zs8Q9tm7DWewip8V28SS8pM94cCP3ykydVTwJV7ULjwsfc4z8ey8fH6HoVZvTEcLPkUuQSvrsuRavpNHijAJ4m13254ugrWg4AmNC6TehHRpMgUNZqKh6bzaNg6S4am77Hr8VDkVjEfCUinkLoCgG3XxadR9UQxcJvb71A1M4aSMGj4VVWshaRLAMk4rEuvJnPmhxtTFhfFRrhwBKRB7qd97jUCwoVVRraK3k2RZvWgLkSMA2LvC5U5GXs5zWguBKm2TW3E9EBmLXNQocJin"
	// This one worked https://explorer.avax-test.network/tx/T8aiWvGywEG9FSeeUodh9cbsbGoC1GEvmPbVSgSfDtTyTxzop
	 txcb58 := "1111111114QVHAgReanECBWVHeabjsijZaBnMuidXUtHQnJ1MU8rKL2HQFfLBoc3BWYyNKFjNZiHLYf6cZuCkiiymGsraX9PMxQA23Tu2yrWyp7GPQ4RjUAMDZwhhksTKNiGings8226QFyP5wiisePa5qhknkigCNTcE34ngFjiGUEEGYbZaePR9uLGr2WyhoEM5Nw9wS2H8kz45CrFaA6FBpRTFmCCh84gCU71CZGddRe3a62cCbUUWhh22KUZbYXZTKpe363Ltod2i8z44yjW812QS4A1YcTdQw8QeLszuv8ZkpkLSgQQxaW6FqgVQBhqrQ5xGsnEFBbxZMjVhbTidLYvzGMsizGPcJWKj8s2r4g93u2gxMMhjHiNSsLSsCSM1EkNNaGbHtcCEYYoPHQQiXZ7hxjUWHjj8pTTmRXWoQUrvaiimJYK3JDLQXGupgTaxhFUa1MFD9SiBvTJA2fGe532eqVwHX6s54CcvsMp9QqZX3AxGZk43S3GuzaSZunixGumXi54YNSYqhL3Pmkn3Z8oNRyCitWz8rsCGz76QT3juTyowgvmdSPcwbuAyE6tgvmfAvPLNjYyyLn8oFjidxfK9Cm89S8CoDzjxEy43rPXhtV6phkckNubjQgRLLMcHWGxHcAzjoDpBJSfhSwzNzQMAib6fpUK2vj9o5JpRcsz7U953FuSQpTw46dwdYoR2vSnQrwEDMr3QxpD4C863fnaT9mTW4oFpYPNFKaR2M4ptZeP5z6yqb7P2DHS1Ft9c2YSSiEPeHnNhpSWhRpiqHPiaRPzyJpaWBUUTZpeWEramh45GygQ9PRXj5FG61mwucx2vnkj5Lmt2c99iQWnoFXbbXxuU5ehn94MXm9EEVyvwSFWtvsCfftQTzKGayYGzZNYpkCjv9VXjz8SuNffsXk6fgmAstyyTH2xQV61ECh69eToEmwSLEBYBL1ckRpz5gv6WzTdV4hoDhRfueuBVZRd51abdYNZHVuGFs8rmPSKZYFGqSrpkonyA26xE2WLb99jqUN9181AgXVJ8LGDmggoGVYzXTwMswoDn8Rw5qrUZY7ohvoaYZ4kXKjGgSurbsd8uQZQBTJ18icAHiKNtBEcPuX9Z6kxzuEAdbCMmyCnZ9KGLJpRyx3tka1pgGsHE6x5tBCW1ADfj6QYeR5SP1UkqTF9kTd8njFVuLd42arWMbXLsd79BXzWN12YHUwpfNSdZztbKm8BMGtZ61EgnHwxhik3Jr8BNni2W1DC762spz973nuQHJhYDfPobL2jhD4HCokX48s7y96uG4ZVdNavbSW2ZFSKyt5YtSBaXcspcf1pcRjJN4cdVWMbj8wZfzCbRenceXKVHdoxc9iUiKai4RguX8uyHcRvANojmaAwuGPic56"
	
	// Faucet drip example https://explorer.avax-test.network/tx/2VgC6RwUhzb18KvsRXJGtDeCeEPhyJ6vkpafenNwWAQPdmT9yZ
	// txcb58 := "111111111BBGmbo1AWTcAfp4oqPvse2QNK5S1Z7t42egPKrexbEWukxFm1yentVNjx9muqHxDz9DAuu2T9Z4qHcQjCYJUmPsuvDPm6GQKBFcTrfRbuBM3KEwG6p2VnqqVKe8RSQLhvV5siHKZkpxmMkA28sUtByyN151pNkxd589ygayqngBjcwtk3AZGftM3gUGu6i5mnyd9DHHGyGWPwqmSxd6BvySZu5Gqn221XetogkqcZvUvfk3x8fD2zMqBtCFLw5bvmPhZAVWoLKdiLoaSHqiBPZ1nXha1cZEgZFLJDLsjamNvgKoJfGScYp7vVGXDQNy4RgQiC2appgS4znEf4nNHFDeLEHz9jru5ieCExMNN6ujjGvatdgp2CGngguckwScggcEvDhLi3Vqtox54tRkYPiH8BJnUJnWwgRrBiqwte54jDzsmp7xhCtENjW31Sa19pJwtufXPyHp1ZEB9rqcu7iwwy1iwRCERLocCnzVo5DRHu9oewNGZQFYLR1ivwkm88x7eaFGhC3b5VDdW"

	// https://explorer.avax-test.network/tx/mqp3iyxdJUxPGkTTvGAZYpA6DZGCBnYiASPRmzmJrccFtUXQ7
	// txcb58 := "111111111UgbbpcKWRe4kWhsrY3aAUd2xUwYYrjWfeSp6g8b9uaE5J35JTCgpYStYfDZp6cGKXv5MrqGWc7urASV1WAusjRvfBwnnmS2SpKoPj6nLbBbvzDgXdR7hNgsQSnsUmryS9bkeSRskmxvZRX3hiVdzFzze1sKEDu9kmjJqa918L6WbrWBkv6i227hWUCoq2miV51wwF2pLXdpbwMwL8gpg8iaeX2kxdiFRhd23KLYUAxdZDiTtzExMkZvYxmQTfyTF2DkTuonfsZZkRK9cKqnwNXBGECxRqqRjH9BGrJfDbNHh8fLhjmt9HgS4SiF92thWFEgy4eh9SQa8hVokGB2nLTXHJPtx9QytV1wRBHk6NJoTx9c5b557GtbKWREDoXcaZHK9trX9vbHrmzovvabYyCJzZPq9QmTeND2NPe8KrPSaE1Zd7eqhJzxvp3e4mYKmpqSCYHs6k5KKKAkMnhjQyh3n2YRxLvWAonut3ZwQwa2om7rDUcxBnu7Fm3CVdV"

	txbytes, err := formatting.Decode(formatting.CB58, txcb58)
	require.NoError(t, err)
	tx := &avm.Tx{UnsignedTx: &avm.BaseTx{BaseTx: avax.BaseTx{
		Metadata:     avax.Metadata{},
		NetworkID:    0,
		BlockchainID: [32]byte{},
		Outs:         []*avax.TransferableOutput{},
		Ins:          []*avax.TransferableInput{},
		Memo:         []byte{},
	}}}
	_, err = w.Codec().Unmarshal(txbytes, tx)
	require.NoError(t, err)

	txid := hashing.ComputeHash256(txbytes)	
	txidcb58, err := formatting.EncodeWithChecksum(formatting.CB58, txid)
	require.NoError(t, err)
	t.Logf("txid: %s", txidcb58)

	sig := tx.Creds[0].Verifiable.(*secp256k1fx.Credential).Sigs[0]
	sigb64 := base64.StdEncoding.EncodeToString(sig[:])
	t.Logf("sigb64: %s  raw sig: %+v", string(sigb64), sig)

	var s secp256k1.ModNScalar
	z := s.SetByteSlice(sig[32:64])
	t.Logf("overflow? %v Odd?: %v", z, s.IsOdd())
	if s.IsOverHalfOrder() {
		t.Log("Mutated")
	}

	f := crypto.FactorySECP256K1R{}
	pubkeyRecovered, err := f.RecoverHashPublicKey(txid, sig[:])
	if err == nil {
		pubkeyRecoveredAva := pubkeyRecovered.(*crypto.PublicKeySECP256K1R)
		verified := pubkeyRecoveredAva.VerifyHash(txid, sig[:])
		t.Logf("Verified: %v", verified)
		pkbytes := pubkeyRecoveredAva.Address().Bytes()
		addr, err := formatting.FormatAddress(w.Config.ChainName,w.Config.NetworkName, pkbytes)
		require.NoError(t, err)
		t.Logf("pubkeyRecoveredAva: %s", addr)

		// if pubkeyRecoveredAva != nil {
		// 	a1 := pubkeyRecoveredAva.Address()
		// 	a2 := w.PublicKeyAvm().Address()
		// 	x := w.PublicKeyAvm().VerifyHash(txid, sig[:])
		// 	t.Log(x)
		// 	y := pubkeyRecoveredAva.VerifyHash(txid, sig[:])
		// 	t.Log(y)
		// 	if a1 == a2 {
		// 		t.Log("OK")
		// 	} else {
		// 		t.Log("NOTOK")
		// 	}
		// }
	} else {
		t.Logf("MpsSigToAvaSig err with Code:   Err: %v", err)
	}



	t.Fatal()
}

func TestFrak(t *testing.T) {
	w := NewEmptyWallet("fuji", "treasury", 1, users["alice"], []user.User{users["bob"], users["cam"]})
	utxocb58 := "11abFWGEAypRbAeP7MxxodGgLYhSEnr72uP8jGuTj8xMcz8bRrbTjxqXJAbg8WBb7XZhTgeXEDTvginxD6mjFHtLbTkqvUY8a7uasF7xM6kTwTRXkRMvhJQs9aUNpx85jE6JRjRVyMWxCBMXPyfSvrr9tN5XQufG1nKK5o"
	utxob, err := formatting.Decode(formatting.CB58, utxocb58)
	require.NoError(t, err)
	utxo := &avax.UTXO{}
	_, err = w.Codec().Unmarshal(utxob, utxo)
	t.Logf("%+v", *utxo)
	// t.Fatal()
}


func TestFujiWallet(t *testing.T) {
	w := NewEmptyWallet("fuji", "treasury", 1, users["alice"], []user.User{users["bob"], users["cam"]})
	if w == nil {
		t.Fatalf("failed to create the new wallet")
	}
	w.Initialize(keydata)

	err := w.FetchUTXOs()
	t.Logf("Balances %+v", w.GetBalances())
	require.NoError(t, err)
	w.DumpUTXOs()
	t.Logf("Balance: %v", w.Balance(w.Config.AssetID))
	require.EqualValuesf(t, uint64(130000000), w.Balance(w.Config.AssetID), "balance not correct")
	
	_, _, b, _ := formatting.ParseAddress(users["alice"].Address)
	aliceaddr, err := ids.ToShortID(b)
	require.NoError(t, err)

	tx, err := w.CreateTx(w.Config.AssetID, uint64(1000), aliceaddr, "memo")
	t.Logf("%+v", tx)

	unsignedBytes, err := w.GetUnsignedBytes(&tx.UnsignedTx)
	if err != nil {
		t.Fatalf("problem creating transaction: %v", err)
	}
	t.Logf("UnsignedBytes: %v", unsignedBytes)

	// // Attach credentials
	// hash := hashing.ComputeHash256(unsignedBytes)
	// for _, keys := range signers {
	// 	cred := &secp256k1fx.Credential{
	// 		Sigs: make([][crypto.SECP256K1RSigLen]byte, len(keys)),
	// 	}
	// 	for i, key := range keys {
	// 		sig, err := key.SignHash(hash) // Sign hash
	// 		if err != nil {
	// 			return fmt.Errorf("problem generating credential: %w", err)
	// 		}
	// 		copy(cred.Sigs[i][:], sig)
	// 	}
	// 	tx.Creds = append(tx.Creds, cred) // Attach credential
	// }

	// signedBytes, err := c.Marshal(codecVersion, tx)
	// if err != nil {
	// 	return fmt.Errorf("couldn't marshal ProposalTx: %w", err)
	// }
	// tx.Initialize(unsignedBytes, signedBytes)
	// return nil

	t.Fatal()
}

// func TestWalletAddUTXO(t *testing.T) {
// 	msk := exampleMultiSigKey()
// 	pk := msk.PublicKeyAVA().(*crypto.PublicKeySECP256K1R)
// 	w, err := NewWallet(appConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	w.keychain.Add(pk)

// 	utxo := &avax.UTXO{
// 		UTXOID: avax.UTXOID{TxID: ids.Empty.Prefix(0)},
// 		Asset:  avax.Asset{ID: appConfig.BC.AssetID},
// 		Out: &secp256k1fx.TransferOutput{
// 			Amt: 1000,
// 			OutputOwners: secp256k1fx.OutputOwners{
// 				Addrs: []ids.ShortID{pk.Address()},
// 				Locktime: 0,
// 				Threshold: 1,
// 			},
// 		},
// 	}

// 	w.AddUTXO(utxo)

// 	if balance := w.Balance(utxo.AssetID()); balance != 1000 {
// 		t.Fatalf("expected balance to be 1000, was %d", balance)
// 	}
// }

// func TestWalletCreateTx(t *testing.T) {
// 	msk := exampleMultiSigKey()
// 	pk := msk.PublicKeyAVA().(*crypto.PublicKeySECP256K1R)
// 	w, err := NewWallet(appConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	w.keychain.Add(pk)

// 	utxo := &avax.UTXO{
// 		UTXOID: avax.UTXOID{TxID: ids.Empty.Prefix(0)},
// 		Asset:  avax.Asset{ID: appConfig.BC.AssetID},
// 		Out: &secp256k1fx.TransferOutput{
// 			Amt: 1000,
// 			OutputOwners: secp256k1fx.OutputOwners{
// 				Addrs: []ids.ShortID{pk.Address()},
// 				Locktime: 0,
// 				Threshold: 1,
// 			},
// 		},
// 	}

// 	w.AddUTXO(utxo)

// 	if balance := w.Balance(utxo.AssetID()); balance != 1000 {
// 		t.Fatalf("expected balance to be 1000, was %d", balance)
// 	}

// 	destAddr := addrs[0]

// 	tx, err := w.CreateBaseTx(appConfig.BC.AssetID, 1000, destAddr, "test memo")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	_, c := setupCodec()

// 	unsignedBytes, err := c.Marshal(codecVersion, &tx.UnsignedTx)
// 	if err != nil {
// 		t.Fatalf("problem creating transaction: %v", err)
// 	}

// 	hash := hashing.ComputeHash256(unsignedBytes)
// 	cb58, err := formatting.EncodeWithChecksum(formatting.CB58, hash)
// 	if err != nil {
// 		t.Fatalf("problem cb58 transaction: %v", err)
// 	}
// 	t.Logf("unsigned bytes: %v", unsignedBytes)
// 	t.Logf("hash: %v", hash)
// 	t.Logf("unsigned cb58: %v", cb58)
// 	// b64sig omFSWCECBiiJFw1v3WJr1gSYJG163R+sRGelst/UmcWckbfHj+dhU1ggMJ8Ylh7ZuY5qNEcVtkMeVsOuCe7db9WxGDnq4XWV6xw=
// 	// b64sig := "omFSWCECBiiJFw1v3WJr1gSYJG163R+sRGelst/UmcWckbfHj+dhU1ggMJ8Ylh7ZuY5qNEcVtkMeVsOuCe7db9WxGDnq4XWV6xw="
// 	sig := []byte{6,40,137,23,13,111,221,98,107,214,4,152,36,109,122,221,31,172,68,103,165,178,223,212,153,197,156,145,183,199,143,231,48,159,24,150,30,217,185,142,106,52,71,21,182,67,30,86,195,174,9,238,221,111,213,177,24,57,234,225,117,149,235,28,0}
// 	sigcb58, _ := formatting.EncodeWithChecksum(formatting.CB58, sig)
// 	t.Logf("sigcb58: %v", sigcb58)

// 	f := crypto.FactorySECP256K1R{}
// 	recv, err := f.RecoverHashPublicKey(hash, sig)
// 	t.Logf("recv: %v", recv.Address())

// 	z, err := formatting.FormatAddress("X","avax", recv.Address().Bytes())
// 	t.Logf("z: %v", z)


// 	cred := &secp256k1fx.Credential{
// 		Sigs: make([][crypto.SECP256K1RSigLen]byte, 1),
// 	}

// 	copy(cred.Sigs[0][:], sig)
// 	tx.Creds = append(tx.Creds, &avm.FxCredential{Verifiable: cred})

// 	signedBytes, err := c.Marshal(codecVersion, tx)
// 	if err != nil {
// 		t.Fatalf("problem creating transaction: %v", err)
// 	}
// 	t.Logf("signed bytes: %v", signedBytes)

// 	tx.Initialize(unsignedBytes, signedBytes)
// 	allBytes, err := c.Marshal(codecVersion, &tx)
// 	if err != nil {
// 		t.Fatalf("problem creating transaction: %v", err)
// 	}
// 	allcb58, err := formatting.EncodeWithChecksum(formatting.CB58, allBytes)
// 	if err != nil {
// 		t.Fatalf("problem cb58 transaction: %v", err)
// 	}
// 	t.Logf("allcb58: %v", allcb58)
	
// 	if balance := w.Balance(utxo.AssetID()); balance != 1000 {
// 		t.Fatalf("expected balance to be 1000, was %d", balance)
// 	}

// 	for _, utxo := range tx.InputUTXOs() {
// 		w.RemoveUTXO(utxo.InputID())
// 	}

// 	if balance := w.Balance(utxo.AssetID()); balance != 0 {
// 		t.Fatalf("expected balance to be 0, was %d", balance)
// 	}


// 	t.Fatal()

// }

// func TestMSKWalletCreateTx(t *testing.T) {
// 	w, err := NewWallet(appConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	assetID := ids.Empty.Prefix(0)

// 	msk := exampleMultiSigKey()
// 	addr := msk.PublicKeyAVA().Address()

// 	utxo := &avax.UTXO{
// 		UTXOID: avax.UTXOID{TxID: ids.Empty.Prefix(1)},
// 		Asset:  avax.Asset{ID: assetID},
// 		Out: &secp256k1fx.TransferOutput{
// 			Amt: 1000,
// 			OutputOwners: secp256k1fx.OutputOwners{
// 				Threshold: 1,
// 				Addrs:     []ids.ShortID{addr},
// 			},
// 		},
// 	}

// 	AddUTXO(w, utxo)

// 	if balance := w.Balance(utxo.AssetID()); balance != 1000 {
// 		t.Fatalf("expected balance to be 1000, was %d", balance)
// 	}

// }


func setupCodec() (codec.GeneralCodec, codec.Manager) {
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
	if errs.Errored() {
		panic(errs.Err)
	}
	return c, m
}

// // func (t *Tx) SignSECP256K1Fx(c codec.Manager, signers [][]*crypto.PrivateKeySECP256K1R) error {
// // 	unsignedBytes, err := c.Marshal(codecVersion, &t.UnsignedTx)
// // 	if err != nil {
// // 		return fmt.Errorf("problem creating transaction: %w", err)
// // 	}

// // 	hash := hashing.ComputeHash256(unsignedBytes)
// // 	for _, keys := range signers {
// // 		cred := &secp256k1fx.Credential{
// // 			Sigs: make([][crypto.SECP256K1RSigLen]byte, len(keys)),
// // 		}
// // 		for i, key := range keys {
// // 			sig, err := key.SignHash(hash)
// // 			if err != nil {
// // 				return fmt.Errorf("problem creating transaction: %w", err)
// // 			}
// // 			copy(cred.Sigs[i][:], sig)
// // 		}
// // 		t.Creds = append(t.Creds, cred)
// // 	}

// // 	signedBytes, err := c.Marshal(codecVersion, t)
// // 	if err != nil {
// // 		return fmt.Errorf("problem creating transaction: %w", err)
// // 	}
// // 	t.Initialize(unsignedBytes, signedBytes)
// // 	return nil
// // }
