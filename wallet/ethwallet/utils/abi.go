package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/shykerbogdan/mpc-wallet/wallet/ethwallet/crypto"
)

func ToMethodID(funcSig string) []byte {
	return crypto.Keccak256([]byte(funcSig))[:4]
}

func EncodeABI(funcSig []byte, inputs ...[]byte) (res []byte) {
	res = append(res, funcSig...)
	for _, input := range inputs {
		temp := AddPrefixZero(input, 32)
		res = append(res, temp...)
	}
	return res
}

func DecodeSingle(str string, typ string) (res interface{}, err error) {
	b := HexStrToBytes(str)
	typs := fmt.Sprintf(`[{ "type": "%s" }]`, typ)
	def := fmt.Sprintf(`[{ "name" : "method", "outputs": %s}]`, typs)
	abi, err := abi.JSON(strings.NewReader(def))
	if err != nil {
		return "", nil
	}
	outptr := reflect.New(reflect.TypeOf(""))
	err = abi.UnpackIntoInterface(outptr.Interface(), "method", b)// UnpackIntoInterface -> unpack
	if err != nil {
		return "", nil
	}
	out := outptr.Elem().Interface()
	return out, nil
}
