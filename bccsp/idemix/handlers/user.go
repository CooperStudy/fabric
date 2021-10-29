/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package handlers

import (
	"crypto/sha256"
	"fmt"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/pkg/errors"
)

// userSecretKey contains the User secret key
type userSecretKey struct {
	// sk is the idemix reference to the User key
	sk Big
	// Exportable if true, sk can be exported via the Bytes function
	exportable bool
}

func NewUserSecretKey(sk Big, exportable bool) *userSecretKey {
	fmt.Println("===NewUserSecretKey=======")
	return &userSecretKey{sk: sk, exportable: exportable}
}

func (k *userSecretKey) Bytes() ([]byte, error) {
	fmt.Println("==userSecretKey=Bytes=======")
	if k.exportable {
		return k.sk.Bytes()
	}

	return nil, errors.New("not exportable")
}

func (k *userSecretKey) SKI() []byte {
	fmt.Println("==userSecretKey=SKI=======")
	raw, err := k.sk.Bytes()
	if err != nil {
		return nil
	}
	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

func (*userSecretKey) Symmetric() bool {
	fmt.Println("==userSecretKey=Symmetric=======")
	return true
}

func (*userSecretKey) Private() bool {
	fmt.Println("==userSecretKey=Private=======")
	return true
}

func (k *userSecretKey) PublicKey() (bccsp.Key, error) {
	fmt.Println("==userSecretKey=PublicKey=======")
	return nil, errors.New("cannot call this method on a symmetric key")
}

type UserKeyGen struct {
	// Exportable is a flag to allow an issuer secret key to be marked as Exportable.
	// If a secret key is marked as Exportable, its Bytes method will return the key's byte representation.
	Exportable bool
	// User implements the underlying cryptographic algorithms
	User User
}

func (g *UserKeyGen) KeyGen(opts bccsp.KeyGenOpts) (bccsp.Key, error) {
	fmt.Println("==UserKeyGen=KeyGen=======")
	sk, err := g.User.NewKey()
	if err != nil {
		return nil, err
	}

	return &userSecretKey{exportable: g.Exportable, sk: sk}, nil
}

// UserKeyImporter import user keys
type UserKeyImporter struct {
	// Exportable is a flag to allow a secret key to be marked as Exportable.
	// If a secret key is marked as Exportable, its Bytes method will return the key's byte representation.
	Exportable bool
	// User implements the underlying cryptographic algorithms
	User User
}

func (i *UserKeyImporter) KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error) {
	fmt.Println("==UserKeyImporter=KeyImport=======")
	der, ok := raw.([]byte)
	if !ok {
		return nil, errors.New("invalid raw, expected byte array")
	}

	if len(der) == 0 {
		return nil, errors.New("invalid raw, it must not be nil")
	}

	sk, err := i.User.NewKeyFromBytes(raw.([]byte))
	if err != nil {
		return nil, err
	}

	return &userSecretKey{exportable: i.Exportable, sk: sk}, nil
}
