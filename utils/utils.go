package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"

	avacrypto "github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/davecgh/go-spew/spew"
	mpsecdsa "github.com/taurusgroup/multi-party-sig/pkg/ecdsa"
)

const (
	SimpleDateFormat    = "Jan 2"
	SimpleTimeFormat    = "15:04 MST"
	MinimumTimeFormat12 = "3:04 PM"
	MinimumTimeFormat24 = "15:04"
	TimestampFormat = "2006-01-02T15:04:05-0700"
)

// DoesNotInclude takes a slice of strings and a target string and returns
// TRUE if the slice does not include the target, FALSE if it does
//
// Example:
//
//    x := DoesNotInclude([]string{"cat", "dog", "rat"}, "dog")
//    > false
//
//    x := DoesNotInclude([]string{"cat", "dog", "rat"}, "pig")
//    > true
//
func DoesNotInclude(strs []string, val string) bool {
	return !Includes(strs, val)
}

// FindMatch takes a regex pattern and a string of data and returns back all the matches
// in that string
func FindMatch(pattern string, data string) [][]string {
	r := regexp.MustCompile(pattern)
	return r.FindAllStringSubmatch(data, -1)
}

// Includes takes a slice of strings and a target string and returns
// TRUE if the slice includes the target, FALSE if it does not
//
// Example:
//
//    x := Includes([]string{"cat", "dog", "rat"}, "dog")
//    > true
//
//    x := Includes([]string{"cat", "dog", "rat"}, "pig")
//    > false
//
func Includes(strs []string, val string) bool {
	for _, str := range strs {
		if val == str {
			return true
		}
	}
	return false
}

func Dump(obj interface{}) string {
	return spew.Sdump(obj)
}

// Fatalf formats a message to standard error and exits the program.
// The message is also printed to standard output if standard error
// is redirected to a different file.
func Fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		}
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}

// ReadFileBytes reads the contents of a file and returns those contents as a slice of bytes
func ReadFileBytes(filePath string) ([]byte, error) {
	fileData, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return []byte{}, err
	}

	return fileData, nil
}

// MustGetUser establishes current user identity or fail.
func MustGetUser() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("Die on retrieving current user info %v", err)
	}
	return usr.Username
}

// EnsurePath ensures a directory exist from the given path.
func EnsurePath(path string, mod os.FileMode) {
	dir := filepath.Dir(path)
	EnsureFullPath(dir, mod)
}

// EnsureFullPath ensures a directory exist from the given path.
func EnsureFullPath(path string, mod os.FileMode) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, mod); err != nil {
			log.Fatalf("Unable to create dir %q %v", path, err)
		}
	}
}


// Same algo as Avax wallet
// msg is the message, returns the hash of the full msg with prefix
func DigestAvaMsg(msg string) []byte {
	msgb := []byte(msg)
	l := uint32(len(msgb))
	lb := make([]byte, 4)
	binary.BigEndian.PutUint32(lb, l)
	prefix := []byte("\x1AAvalanche Signed Message:\n")

	buf := new(bytes.Buffer)
	buf.Write(prefix)
	buf.Write(lb)
	buf.Write(msgb)
	fullmsg := buf.Bytes()
	h := sha256.Sum256(fullmsg)
	return h[:]
}

// Validate a msg signed with the Avalanche web wallet "Sign Message" feature
func PublicKeyFromAvaMsg(msg string, avasigcb58 string) (avacrypto.PublicKey, error) {
	h := DigestAvaMsg(msg)

	avasig, err := formatting.Decode(formatting.CB58, avasigcb58)
	if err != nil {
		return nil, err
	}

	f := avacrypto.FactorySECP256K1R{}
	pk, err := f.RecoverHashPublicKey(h, avasig)
	return pk, err
}

// Useful for using standard ecdsa funcs that require int args
func SigToRS(sig mpsecdsa.Signature) (big.Int, big.Int) {
	var si big.Int
	b, _ := sig.S.MarshalBinary()
	si.SetBytes(b[:])

	var ri big.Int
	b, _ = sig.R.XScalar().MarshalBinary()
	ri.SetBytes(b[:])

	return ri, si
}

