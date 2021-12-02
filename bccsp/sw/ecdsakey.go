/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package sw

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/bccsp"
)

type ecdsaPrivateKey struct {
	privKey *ecdsa.PrivateKey
}

// Bytes converts this key to its byte representation,
// if this operation is allowed.
func (k *ecdsaPrivateKey) Bytes() ([]byte, error) {
	logger.Info("===ecdsaPrivateKey==Bytes===")
	return nil, errors.New("Not supported.")
}

// SKI returns the subject key identifier of this key.
func (k *ecdsaPrivateKey) SKI() []byte {
	logger.Info("===ecdsaPrivateKey==SKI===")
	if k.privKey == nil {
		return nil
	}

	// Marshall the public key
	raw := elliptic.Marshal(k.privKey.Curve, k.privKey.PublicKey.X, k.privKey.PublicKey.Y)

	// Hash it
	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

// Symmetric returns true if this key is a symmetric key,
// false if this key is asymmetric
func (k *ecdsaPrivateKey) Symmetric() bool {
	logger.Info("===ecdsaPrivateKey==Symmetric===")
	return false
}

// Private returns true if this key is a private key,
// false otherwise.
func (k *ecdsaPrivateKey) Private() bool {
	logger.Info("===ecdsaPrivateKey==Private===")
	return true
}

// PublicKey returns the corresponding public key part of an asymmetric public/private key pair.
// This method returns an error in symmetric key schemes.
func (k *ecdsaPrivateKey) PublicKey() (bccsp.Key, error) {
	logger.Info("===ecdsaPrivateKey==PublicKey===")
	return &ecdsaPublicKey{&k.privKey.PublicKey}, nil
}

type ecdsaPublicKey struct {
	pubKey *ecdsa.PublicKey
}

// Bytes converts this key to its byte representation,
// if this operation is allowed.
func (k *ecdsaPublicKey) Bytes() (raw []byte, err error) {
	logger.Info("===ecdsaPublicKey==Bytes===")
	raw, err = x509.MarshalPKIXPublicKey(k.pubKey)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling key [%s]", err)
	}
	return
}

// SKI returns the subject key identifier of this key.
func (k *ecdsaPublicKey) SKI() []byte {
	logger.Info("===ecdsaPublicKey==SKI===")
	if k.pubKey == nil {
		return nil
	}

	// Marshall the public key
	raw := elliptic.Marshal(k.pubKey.Curve, k.pubKey.X, k.pubKey.Y)

	// Hash it
	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

// Symmetric returns true if this key is a symmetric key,
// false if this key is asymmetric
func (k *ecdsaPublicKey) Symmetric() bool {
	logger.Info("===ecdsaPublicKey==Symmetric===")
	return false
}

// Private returns true if this key is a private key,
// false otherwise.
func (k *ecdsaPublicKey) Private() bool {
	logger.Info("===ecdsaPublicKey==Private===")
	return false
}

// PublicKey returns the corresponding public key part of an asymmetric public/private key pair.
// This method returns an error in symmetric key schemes.
func (k *ecdsaPublicKey) PublicKey() (bccsp.Key, error) {
	logger.Info("===ecdsaPublicKey==PublicKey===")
	return k, nil
}
