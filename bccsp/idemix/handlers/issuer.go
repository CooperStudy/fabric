/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package handlers

import (
	"fmt"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/pkg/errors"
)

// issuerSecretKey contains the issuer secret key
// and implements the bccsp.Key interface
type issuerSecretKey struct {
	// sk is the idemix reference to the issuer key
	sk IssuerSecretKey
	// exportable if true, sk can be exported via the Bytes function
	exportable bool
}

func NewIssuerSecretKey(sk IssuerSecretKey, exportable bool) *issuerSecretKey {
	fmt.Println("=====NewIssuerSecretKey=================")
	return &issuerSecretKey{sk: sk, exportable: exportable}
}

func (k *issuerSecretKey) Bytes() ([]byte, error) {
	fmt.Println("====issuerSecretKey=Bytes=================")
	if k.exportable {
		return k.sk.Bytes()
	}

	return nil, errors.New("not exportable")
}

func (k *issuerSecretKey) SKI() []byte {
	fmt.Println("====issuerSecretKey=SKI=================")
	pk, err := k.PublicKey()
	if err != nil {
		return nil
	}

	return pk.SKI()
}

func (*issuerSecretKey) Symmetric() bool {
	fmt.Println("====issuerSecretKey=Symmetric=================")
	return false
}

func (*issuerSecretKey) Private() bool {
	fmt.Println("====issuerSecretKey=Private=================")
	return true
}

func (k *issuerSecretKey) PublicKey() (bccsp.Key, error) {
	fmt.Println("====issuerSecretKey=PublicKey=================")
	return &issuerPublicKey{k.sk.Public()}, nil
}

// issuerPublicKey contains the issuer public key
// and implements the bccsp.Key interface
type issuerPublicKey struct {
	pk IssuerPublicKey
}

func NewIssuerPublicKey(pk IssuerPublicKey) *issuerPublicKey {
	fmt.Println("====NewIssuerPublicKey=================")
	return &issuerPublicKey{pk}
}

func (k *issuerPublicKey) Bytes() ([]byte, error) {
	fmt.Println("====issuerPublicKey====Bytes=============")
	return k.pk.Bytes()
}

func (k *issuerPublicKey) SKI() []byte {
	fmt.Println("====issuerPublicKey====SKI=============")
	return k.pk.Hash()
}

func (*issuerPublicKey) Symmetric() bool {
	fmt.Println("====issuerPublicKey====Symmetric=============")
	return false
}

func (*issuerPublicKey) Private() bool {
	fmt.Println("====issuerPublicKey====Private=============")
	return false
}

func (k *issuerPublicKey) PublicKey() (bccsp.Key, error) {
	fmt.Println("====issuerPublicKey====PublicKey=============")
	return k, nil
}

// IssuerKeyGen generates issuer secret keys.
type IssuerKeyGen struct {
	// exportable is a flag to allow an issuer secret key to be marked as exportable.
	// If a secret key is marked as exportable, its Bytes method will return the key's byte representation.
	Exportable bool
	// Issuer implements the underlying cryptographic algorithms
	Issuer Issuer
}

func (g *IssuerKeyGen) KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error) {
	fmt.Println("====IssuerKeyGen====KeyGen=============")
	o, ok := opts.(*bccsp.IdemixIssuerKeyGenOpts)
	if !ok {
		return nil, errors.New("invalid options, expected *bccsp.IdemixIssuerKeyGenOpts")
	}

	// Create a new key pair
	key, err := g.Issuer.NewKey(o.AttributeNames)
	if err != nil {
		return nil, err
	}

	return &issuerSecretKey{exportable: g.Exportable, sk: key}, nil
}

// IssuerPublicKeyImporter imports issuer public keys
type IssuerPublicKeyImporter struct {
	// Issuer implements the underlying cryptographic algorithms
	Issuer Issuer
}

func (i *IssuerPublicKeyImporter) KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error) {
	fmt.Println("====IssuerPublicKeyImporter====KeyImport=============")
	der, ok := raw.([]byte)
	if !ok {
		return nil, errors.New("invalid raw, expected byte array")
	}

	if len(der) == 0 {
		return nil, errors.New("invalid raw, it must not be nil")
	}

	o, ok := opts.(*bccsp.IdemixIssuerPublicKeyImportOpts)
	if !ok {
		return nil, errors.New("invalid options, expected *bccsp.IdemixIssuerPublicKeyImportOpts")
	}

	pk, err := i.Issuer.NewPublicKeyFromBytes(raw.([]byte), o.AttributeNames)
	if err != nil {
		return nil, err
	}

	return &issuerPublicKey{pk}, nil
}
