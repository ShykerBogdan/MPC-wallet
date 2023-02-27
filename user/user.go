package user

import (
	"encoding/json"
	"fmt"

	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
)

type User struct {
	Nick string
	Address string
	// All users will have a pub key used for identification in libp2p
	IdentPubKey libp2pcrypto.PubKey
}

func (u User) MarshalJSON() ([]byte, error) {
	// We have to marshal an empty User at some points (TODO fix) so support it
	var identpubkey_encoded string
	if u.IdentPubKey != nil {
		identpubkey, err := libp2pcrypto.MarshalPublicKey(u.IdentPubKey)
		if err != nil {
			return []byte{}, fmt.Errorf("error marshaling user IdentPubKey %v", err)
		}
		identpubkey_encoded = libp2pcrypto.ConfigEncodeKey(identpubkey)
	}

	return json.Marshal(struct{
		Nick string
		Address string
		IdentPubKey string
	}{
		Nick: u.Nick,
		Address: u.Address,
		IdentPubKey: identpubkey_encoded,
	})
}

// TODO Please let there be a nicer way to do this
func (u *User) UnmarshalJSON(b []byte) error {
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
	if identpubkeyb64 != "" {
		identpubkeybytes, err := libp2pcrypto.ConfigDecodeKey(identpubkeyb64)
		if err != nil {
			return err		
		}
		identpubkey, err := libp2pcrypto.UnmarshalPublicKey(identpubkeybytes)
		if err != nil {
			return err		
		}
		u.IdentPubKey = identpubkey
	}

	return nil
}

func NewUser(nick string, address string, identpubkey libp2pcrypto.PubKey) (User, error) {
	u := User{
		Nick: nick,
		Address: address,
		IdentPubKey: identpubkey,
	}
	return u, nil
}

func (u User) PeerID() peer.ID {
	pid, err := peer.IDFromPublicKey(u.IdentPubKey)
	if err != nil {
		panic(err)
	}
	return pid
}

// The MSP protocol lib requires each user to have a unique Party ID represented as an exactly 32 byte string
// Since each user has a pub key, which is used as a peer id in libp2p, lets use that.
// Marshal the key to bytes, convert to Pretty() format, then take last 32 characters.
func (u User) PartyID() party.ID {
	pid, err := peer.IDFromPublicKey(u.IdentPubKey)
	if err != nil {
		panic(err)
	}
	return party.ID(pid.Pretty()[(len(pid.Pretty())-32):])
}

// TODO how to do this? verify that a particular nick verified somehow, use libp2p pub/priv key?
func (u User) IsVerified() bool {
	return true
	// msg := fmt.Sprintf("%s:%s", u.Chatroom, u.Nick)
	// pk, err := utils.PublicKeyFromAvaMsg(msg, u.SignedNick)
	// if err != nil {
	// 	return false
	// }
	// _, _, b, _ := formatting.ParseAddress(u.Address)
	// result := bytes.Compare(b, pk.Address().Bytes())
	// return result == 0
}

