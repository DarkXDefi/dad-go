package httpjsonrpc

import (
	"dad-go/client"
	. "dad-go/common"
	"dad-go/common/log"
	. "dad-go/core/asset"
	"dad-go/core/contract"
	"dad-go/core/ledger"
	"dad-go/core/signature"
	"dad-go/core/transaction"
	"dad-go/core/validation"
	"errors"
	"strconv"
)

const (
	ASSETPREFIX = "dad-go"
)

func NewRegTx(rand string, index int, admin, issuer *client.Account) *transaction.Transaction {
	name := ASSETPREFIX + "-" + strconv.Itoa(index) + "-" + rand
	asset := &Asset{name, byte(0x00), AssetType(Share), UTXO}
	amount := Fixed64(1000)
	controller, _ := contract.CreateSignatureContract(admin.PubKey())
	tx, _ := transaction.NewRegisterAssetTransaction(asset, amount, issuer.PubKey(), controller.ProgramHash)
	return tx
}

func SignTx(admin *client.Account, tx *transaction.Transaction) {
	signdate, err := signature.SignBySigner(tx, admin)
	if err != nil {
		log.Error(err, "signdate SignBySigner failed")
	}
	transactionContract, _ := contract.CreateSignatureContract(admin.PublicKey)
	transactionContractContext := contract.NewContractContext(tx)
	transactionContractContext.AddContract(transactionContract, admin.PublicKey, signdate)
	tx.SetPrograms(transactionContractContext.GetPrograms())
}

func SendTx(tx *transaction.Transaction) error {
	if err := validation.VerifyTransaction(tx); err != nil {
		log.Error("Transaction verification failed")
	}
	if err := validation.VerifyTransactionWithLedger(tx, ledger.DefaultLedger); err != nil {
		log.Error("Transaction verification with ledger failed")
	}
	if !node.AppendTxnPool(tx) {
		log.Warn("Can NOT add the transaction to TxnPool")
		return errors.New("Add to transaction pool failed")
	}
	if err := node.Xmit(tx); err != nil {
		log.Error("Xmit Tx Error")
		return errors.New("Xmit transaction failed")
	}
	return nil
}
