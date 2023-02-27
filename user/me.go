package user

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
)

type Me struct {
	User
	// The Me user will also have a priv key, used by libp2p to sign messages
	IdentPrivKey libp2pcrypto.PrivKey
}

func (u Me) MarshalJSON() ([]byte, error) {
	identpubkey, err := libp2pcrypto.MarshalPublicKey(u.IdentPubKey)
	if err != nil {
		return []byte{}, fmt.Errorf("error marshaling user IdentPubKey %v", err)
	}

	identprivkey, err := libp2pcrypto.MarshalPrivateKey(u.IdentPrivKey)
	if err != nil {
		return []byte{}, fmt.Errorf("error marshaling user IdentPrivKey %v", err)
	}
	return json.Marshal(map[string]string{
		"Nick": u.Nick,
		"Address": u.Address,
		"IdentPubKey": libp2pcrypto.ConfigEncodeKey(identpubkey),
		"IdentPrivKey": libp2pcrypto.ConfigEncodeKey(identprivkey),
	})	
}

// TODO Please let there be a nicer way to do this
func (u *Me) UnmarshalJSON(b []byte) error {
	var objMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &objMap)
	if err != nil {
		return err		
	}

	var nick string
	err = json.Unmarshal(*objMap["Nick"], &nick)
	if err != nil {
		return err		
	}
	u.Nick = nick

	var address string
	err = json.Unmarshal(*objMap["Address"], &address)
	if err != nil {
		return err		
	}
	u.Address = address

	var identpubkeyb64 string
	err = json.Unmarshal(*objMap["IdentPubKey"], &identpubkeyb64)
	if err != nil {
		return err		
	}
	identpubkeybytes, err := libp2pcrypto.ConfigDecodeKey(identpubkeyb64)
	if err != nil {
		return err		
	}
	identpubkey, err := libp2pcrypto.UnmarshalPublicKey(identpubkeybytes)
	if err != nil {
		return err		
	}
	u.IdentPubKey = identpubkey

	var identprivkeyb64 string
	err = json.Unmarshal(*objMap["IdentPrivKey"], &identprivkeyb64)
	if err != nil {
		return err		
	}
	identprivkeybytes, err := libp2pcrypto.ConfigDecodeKey(identprivkeyb64)
	if err != nil {
		return err		
	}
	identprivkey, err := libp2pcrypto.UnmarshalPrivateKey(identprivkeybytes)
	if err != nil {
		return err		
	}
	u.IdentPrivKey = identprivkey

	return nil
}

// Create a user which is ourselves
func NewMe(nick string, address string) (Me, error) {
	// If we are creating ourselves, we wont have a identpubkey yet, so create our libp2p identity which is a pub/priv key
	privkey, pubkey, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.RSA, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}

	u := Me{
		User: User{
			Nick: nick,
			Address: address,
			IdentPubKey: pubkey,
		},
		IdentPrivKey: privkey,
	}
	return u, nil
}

