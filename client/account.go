package client

import (
	"dad-go/crypto"
	. "dad-go/common"
	. "dad-go/errors"
	_ "errors"
)

type Account struct {
	PrivateKey []byte
	PublicKey *crypto.PubKey
	PublicKeyHash Uint160
}

func NewAccount(privateKey []byte) (*Account, error){
	//privKeyLen := len(privateKey)
	//
	//if privKeyLen != 32 && privKeyLen != 96 && privKeyLen != 104 {
	//	return nil,errors.New("Invalid private Key.")
	//}
	//
	//priKey := make([]byte,32)
	//pubKey := &crypto.PubKey{}

	//TODO: copy private Key

	//TODO: set public key
	priKey,pubKey,_ := crypto.GenKeyPair()
	temp,err := pubKey.EncodePoint(true)
	if err !=nil{
		return nil,NewDetailErr(err, ErrNoCode, "[Contract],CreateSignatureContract failed.")
	}
	return &Account{
		PrivateKey: priKey,
		PublicKey: &pubKey,
		PublicKeyHash: ToCodeHash(temp),
	},nil
}

//get signer's private key
func (ac *Account) PrivKey() []byte{
	return ac.PrivateKey
}

//get signer's public key
func (ac *Account) PubKey() *crypto.PubKey {
	return ac.PublicKey
}
