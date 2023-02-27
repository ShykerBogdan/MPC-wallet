package user

import (
	"crypto/rand"
	"testing"

	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
)

func TestMeMarshal(t *testing.T) {
	u, err := NewMe("alice", "X-fuji13avtfecrzkhxrd8mxqcd0ehctsvqh99y6xjnr2")
	require.NoError(t, err)
	_, err = u.MarshalJSON()
	require.NoError(t, err)
}

func TestUserMarshal(t *testing.T) {
	_, pubkey, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	u, err := NewUser("alice", "X-fuji13avtfecrzkhxrd8mxqcd0ehctsvqh99y6xjnr2", pubkey)
	require.NoError(t, err)
	_, err = u.MarshalJSON()
	require.NoError(t, err)
}