package ledger

import (
	. "dad-go/common"
	tx "dad-go/core/transaction"
	"dad-go/crypto"
	. "dad-go/errors"
	"errors"
)

var DefaultLedger *Ledger

// Ledger - the struct for onchainDNA ledger
type Ledger struct {
	Blockchain *Blockchain
	State      *State
	Store      ILedgerStore
}

func (l *Ledger) IsDoubleSpend(Tx *tx.Transaction) error {
	//TODO: implement ledger IsDoubleSpend

	return nil
}

func GetDefaultLedger() (*Ledger, error) {
	if DefaultLedger == nil {
		return nil, NewDetailErr(errors.New("[Ledger], GetDefaultLedger failed, DefaultLedger not Exist."), ErrNoCode, "")
	}
	return DefaultLedger, nil
}

func GetMinerAddress(miners []*crypto.PubKey) Uint160 {
	//TODO: GetMinerAddress()
	return Uint160{}
}
