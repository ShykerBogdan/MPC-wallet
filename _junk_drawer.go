func TestCB58(t *testing.T) {
	appConfig := Load("../alice.json")
	w := appConfig.FindWallet("k1")
	msgHashcb58 := "2rgdM1yLoDEegpGA5fxGFydwL5fZhJEY3bqz2Ara4kvt8NVmRU"
	msgHash, _ := formatting.Decode(formatting.CB58, msgHashcb58)
	avasigcb58 := "2M6LnT1FXGprkBwLYSyKTkTbL6Ajn3ZNZkRqqXDYtCg2QBDiBwPbV884oPtY5u7iDaFjw7ommu9MhiN1EiL5jdRrRDnbLQb"
	avasig, _ := formatting.Decode(formatting.CB58, avasigcb58)
	pubkeyRecovered, err := utils.AvaRecoverHashPublicKey(msgHash, avasig[:])
	if err == nil {
		pubkeyRecoveredAva := pubkeyRecovered.(*avacrypto.PublicKeySECP256K1R)
		if pubkeyRecoveredAva != nil {
			a1 := pubkeyRecoveredAva.Address()
			t.Log(a1)
			a2 := w.PublicKeyAvm().Address()
			t.Log(a2)
			if a1 == a2 {
				t.Log("match!")
			}
		}
	} else {
		t.Logf("MpsSigToAvaSig err with Err: %v", err)
	}
	t.Fatal()
}

func TestAvaRecoverableSigs(t *testing.T) {
	appConfig := Load("../alice.json")
	w := appConfig.FindWallet("k1")
	w.PublicKeyAvm().Address()
	// hashedMsg := []byte{244,130,7,3,150,12,185,188,204,72,214,253,195,90,211,71,125,110,81,180,138,75,2,83,6,86,75,102,62,105,44,184}
	// mpssigR   := []byte{51,8,140,230,37,78,220,237,180,69,163,217,192,152,222,185,84,165,167,153,34,22,170,191,106,189,72,137,141,127,13,67}
	// mpssigS   := []byte{57,192,149,94,58,94,125,249,179,103,240,208,182,164,209,214,1,178,194,86,93,131,225,4,127,2,0,120,173,235,161,149}

	hashedMsg := []byte{7,127,72,41,192,24,135,141,236,239,255,84,228,104,50,180,182,251,78,247,135,106,228,41,123,28,130,22,200,136,79,231}
	mpssigR := []byte{115,55,192,95,152,218,136,248,126,188,187,212,63,212,113,171,144,100,168,85,202,42,116,190,230,162,64,116,65,134,242,82}
	mpssigS := []byte{50,15,37,163,65,100,204,107,146,132,48,25,81,36,192,148,56,102,75,221,112,51,216,219,111,190,60,80,133,198,137,127}

	var sigava [65]byte
	copy(sigava[0:32], mpssigR[0:32])
	copy(sigava[32:64], mpssigS[0:32])

	// Try all the recovery codes and see which one is the correct one
	codes := []byte{0, 1, 2, 3, 4}
	for _, c := range codes {
		sigava[64] = c
		// pubkeyRecovered, err := utils.AvaRecoverHashPublicKey(hashedMsg, sigava[:])
		f := avacrypto.FactorySECP256K1R{}
		pubkeyRecovered, err := f.RecoverHashPublicKey(hashedMsg, sigava[:])
		if err == nil {
			pubkeyRecoveredAva, ok := pubkeyRecovered.(*avacrypto.PublicKeySECP256K1R)
			if !ok {
				t.Log("NOT OK")				
			}
			if pubkeyRecoveredAva != nil {
				a1 := pubkeyRecoveredAva.Address()
				a2 := w.PublicKeyAvm().Address()
				t.Logf("c: %v a1: %v", c, a1)
				t.Logf("c: %v a2: %v", c, a2)
				if a1 == a2 {
					t.Logf("MATCHED! %v", w.Address)
				}
			}
		} else {
			t.Logf("MpsSigToAvaSig err with Code: %v  Err: %v", c, err)
		}
	}
	t.Fatal()
}

// From MPS lib, ToRS returns R, S such that ecdsa.Verify(pub,message, R, S) == true.
func (sig Signature) ToRS() (*big.Int, *big.Int) {
	return sig.R.XScalar().BigInt(), sig.S.BigInt()
}
