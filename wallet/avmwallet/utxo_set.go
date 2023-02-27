package avmwallet

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type UTXOSet struct {
	// Key: The id of a UTXO
	// Value: The index in UTXOs of that UTXO
	utxoMap map[ids.ID]int

	// List of UTXOs in this set
	// This can be used to iterate over. It should not be modified externally.
	UTXOs []*avax.UTXO
}

// Return UTXOs sorted smallest to largest amounts. This is so new txs will spend dust first.
func (us *UTXOSet) Sorted() []*avax.UTXO {
	sorted := []*avax.UTXO{}
	for _, utxo := range us.UTXOs {
		_, ok := utxo.Out.(*secp256k1fx.TransferOutput)
		if ok {
			sorted = append(sorted, utxo)
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		iout := sorted[i].Out.(*secp256k1fx.TransferOutput)
		jout := sorted[j].Out.(*secp256k1fx.TransferOutput)
		return iout.Amt < jout.Amt
	})

	return sorted
}

func (us *UTXOSet) Put(utxo *avax.UTXO) {
	if us.utxoMap == nil {
		us.utxoMap = make(map[ids.ID]int)
	}
	utxoID := utxo.InputID()
	if _, ok := us.utxoMap[utxoID]; !ok {
		us.utxoMap[utxoID] = len(us.UTXOs)
		us.UTXOs = append(us.UTXOs, utxo)
	}
}

func (us *UTXOSet) Get(id ids.ID) *avax.UTXO {
	if us.utxoMap == nil {
		return nil
	}
	if i, ok := us.utxoMap[id]; ok {
		utxo := us.UTXOs[i]
		return utxo
	}
	return nil
}

func (us *UTXOSet) Remove(id ids.ID) *avax.UTXO {
	i, ok := us.utxoMap[id]
	if !ok {
		return nil
	}
	utxoI := us.UTXOs[i]

	j := len(us.UTXOs) - 1
	utxoJ := us.UTXOs[j]

	us.UTXOs[i] = us.UTXOs[j]
	us.UTXOs = us.UTXOs[:j]

	us.utxoMap[utxoJ.InputID()] = i
	delete(us.utxoMap, utxoI.InputID())

	return utxoI
}

// PrefixedString returns a string with each new line prefixed with [prefix]
func (us *UTXOSet) PrefixedString(prefix string) string {
	s := strings.Builder{}

	s.WriteString(fmt.Sprintf("UTXOs (length=%d):", len(us.UTXOs)))
	for i, utxo := range us.UTXOs {
		utxoID := utxo.InputID()
		txID, txIndex := utxo.InputSource()

		s.WriteString(fmt.Sprintf("\n%sUTXO[%d]:"+
			"\n%s    UTXOID: %s"+
			"\n%s    TxID: %s"+
			"\n%s    TxIndex: %d",
			prefix, i,
			prefix, utxoID,
			prefix, txID,
			prefix, txIndex))
	}

	return s.String()
}

func (us *UTXOSet) String() string { return us.PrefixedString("  ") }
