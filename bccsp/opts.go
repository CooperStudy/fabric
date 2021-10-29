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

package bccsp

import "fmt"

const (
	// ECDSA Elliptic Curve Digital Signature Algorithm (key gen, import, sign, verify),
	// at default security level.
	// Each BCCSP may or may not support default security level. If not supported than
	// an error will be returned.
	ECDSA = "ECDSA"

	// ECDSA Elliptic Curve Digital Signature Algorithm over P-256 curve
	ECDSAP256 = "ECDSAP256"

	// ECDSA Elliptic Curve Digital Signature Algorithm over P-384 curve
	ECDSAP384 = "ECDSAP384"

	// ECDSAReRand ECDSA key re-randomization
	ECDSAReRand = "ECDSA_RERAND"

	// RSA at the default security level.
	// Each BCCSP may or may not support default security level. If not supported than
	// an error will be returned.
	RSA = "RSA"
	// RSA at 1024 bit security level.
	RSA1024 = "RSA1024"
	// RSA at 2048 bit security level.
	RSA2048 = "RSA2048"
	// RSA at 3072 bit security level.
	RSA3072 = "RSA3072"
	// RSA at 4096 bit security level.
	RSA4096 = "RSA4096"

	// AES Advanced Encryption Standard at the default security level.
	// Each BCCSP may or may not support default security level. If not supported than
	// an error will be returned.
	AES = "AES"
	// AES Advanced Encryption Standard at 128 bit security level
	AES128 = "AES128"
	// AES Advanced Encryption Standard at 192 bit security level
	AES192 = "AES192"
	// AES Advanced Encryption Standard at 256 bit security level
	AES256 = "AES256"

	// HMAC keyed-hash message authentication code
	HMAC = "HMAC"
	// HMACTruncated256 HMAC truncated at 256 bits.
	HMACTruncated256 = "HMAC_TRUNCATED_256"

	// SHA Secure Hash Algorithm using default family.
	// Each BCCSP may or may not support default security level. If not supported than
	// an error will be returned.
	SHA = "SHA"

	// SHA2 is an identifier for SHA2 hash family
	SHA2 = "SHA2"
	// SHA3 is an identifier for SHA3 hash family
	SHA3 = "SHA3"

	// SHA256
	SHA256 = "SHA256"
	// SHA384
	SHA384 = "SHA384"
	// SHA3_256
	SHA3_256 = "SHA3_256"
	// SHA3_384
	SHA3_384 = "SHA3_384"

	// X509Certificate Label for X509 certificate related operation
	X509Certificate = "X509Certificate"
)

// ECDSAKeyGenOpts contains options for ECDSA key generation.
type ECDSAKeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *ECDSAKeyGenOpts) Algorithm() string {
	fmt.Println("===ECDSAKeyGenOpts==Algorithm===")
	return ECDSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAKeyGenOpts) Ephemeral() bool {
	fmt.Println("===ECDSAKeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// ECDSAPKIXPublicKeyImportOpts contains options for ECDSA public key importation in PKIX format
type ECDSAPKIXPublicKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *ECDSAPKIXPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===ECDSAPKIXPublicKeyImportOpts==Algorithm===")
	return ECDSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAPKIXPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===ECDSAPKIXPublicKeyImportOpts==Ephemeral===")
	return opts.Temporary
}

// ECDSAPrivateKeyImportOpts contains options for ECDSA secret key importation in DER format
// or PKCS#8 format.
type ECDSAPrivateKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *ECDSAPrivateKeyImportOpts) Algorithm() string {
	fmt.Println("===ECDSAPrivateKeyImportOpts==Algorithm===")
	return ECDSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAPrivateKeyImportOpts) Ephemeral() bool {
	fmt.Println("===ECDSAPrivateKeyImportOpts==Ephemeral===")
	return opts.Temporary
}

// ECDSAGoPublicKeyImportOpts contains options for ECDSA key importation from ecdsa.PublicKey
type ECDSAGoPublicKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *ECDSAGoPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===ECDSAGoPublicKeyImportOpts==Algorithm===")
	return ECDSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAGoPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===ECDSAGoPublicKeyImportOpts==Ephemeral===")
	return opts.Temporary
}

// ECDSAReRandKeyOpts contains options for ECDSA key re-randomization.
type ECDSAReRandKeyOpts struct {
	Temporary bool
	Expansion []byte
}

// Algorithm returns the key derivation algorithm identifier (to be used).
func (opts *ECDSAReRandKeyOpts) Algorithm() string {
	fmt.Println("===ECDSAReRandKeyOpts==Algorithm===")
	return ECDSAReRand
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAReRandKeyOpts) Ephemeral() bool {
	fmt.Println("===ECDSAReRandKeyOpts==Ephemeral===")
	return opts.Temporary
}

// ExpansionValue returns the re-randomization factor
func (opts *ECDSAReRandKeyOpts) ExpansionValue() []byte {
	fmt.Println("===ECDSAReRandKeyOpts==ExpansionValue===")
	return opts.Expansion
}

// AESKeyGenOpts contains options for AES key generation at default security level
type AESKeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *AESKeyGenOpts) Algorithm() string {
	fmt.Println("===AESKeyGenOpts==Algorithm===")
	return AES
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *AESKeyGenOpts) Ephemeral() bool {
	fmt.Println("===AESKeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// HMACTruncated256AESDeriveKeyOpts contains options for HMAC truncated
// at 256 bits key derivation.
type HMACTruncated256AESDeriveKeyOpts struct {
	Temporary bool
	Arg       []byte
}

// Algorithm returns the key derivation algorithm identifier (to be used).
func (opts *HMACTruncated256AESDeriveKeyOpts) Algorithm() string {
	fmt.Println("===HMACTruncated256AESDeriveKeyOpts==Algorithm===")
	return HMACTruncated256
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *HMACTruncated256AESDeriveKeyOpts) Ephemeral() bool {
	fmt.Println("===HMACTruncated256AESDeriveKeyOpts==Ephemeral===")
	return opts.Temporary
}

// Argument returns the argument to be passed to the HMAC
func (opts *HMACTruncated256AESDeriveKeyOpts) Argument() []byte {
	fmt.Println("===HMACTruncated256AESDeriveKeyOpts==Argument===")
	return opts.Arg
}

// HMACDeriveKeyOpts contains options for HMAC key derivation.
type HMACDeriveKeyOpts struct {
	Temporary bool
	Arg       []byte
}

// Algorithm returns the key derivation algorithm identifier (to be used).
func (opts *HMACDeriveKeyOpts) Algorithm() string {
	fmt.Println("===HMACDeriveKeyOpts==Algorithm===")
	return HMAC
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *HMACDeriveKeyOpts) Ephemeral() bool {
	fmt.Println("===HMACDeriveKeyOpts==Ephemeral===")
	return opts.Temporary
}

// Argument returns the argument to be passed to the HMAC
func (opts *HMACDeriveKeyOpts) Argument() []byte {
	fmt.Println("===HMACDeriveKeyOpts==Argument===")
	return opts.Arg
}

// AES256ImportKeyOpts contains options for importing AES 256 keys.
type AES256ImportKeyOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *AES256ImportKeyOpts) Algorithm() string {
	fmt.Println("===AES256ImportKeyOpts==Algorithm===")
	return AES
}

// Ephemeral returns true if the key generated has to be ephemeral,
// false otherwise.
func (opts *AES256ImportKeyOpts) Ephemeral() bool {
	fmt.Println("===AES256ImportKeyOpts==Ephemeral===")
	return opts.Temporary
}

// HMACImportKeyOpts contains options for importing HMAC keys.
type HMACImportKeyOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *HMACImportKeyOpts) Algorithm() string {
	fmt.Println("===HMACImportKeyOpts==Algorithm===")
	return HMAC
}

// Ephemeral returns true if the key generated has to be ephemeral,
// false otherwise.
func (opts *HMACImportKeyOpts) Ephemeral() bool {
	fmt.Println("===HMACImportKeyOpts==Ephemeral===")
	return opts.Temporary
}

// SHAOpts contains options for computing SHA.
type SHAOpts struct {
}

// Algorithm returns the hash algorithm identifier (to be used).
func (opts *SHAOpts) Algorithm() string {
	fmt.Println("===SHAOpts==Algorithm===")
	return SHA
}

// RSAKeyGenOpts contains options for RSA key generation.
type RSAKeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *RSAKeyGenOpts) Algorithm() string {
	fmt.Println("===RSAKeyGenOpts==Algorithm===")
	return RSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSAKeyGenOpts) Ephemeral() bool {
	fmt.Println("===RSAKeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// ECDSAGoPublicKeyImportOpts contains options for RSA key importation from rsa.PublicKey
type RSAGoPublicKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *RSAGoPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===RSAGoPublicKeyImportOpts==Algorithm===")
	return RSA
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSAGoPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===RSAGoPublicKeyImportOpts==Ephemeral===")
	return opts.Temporary
}

// X509PublicKeyImportOpts contains options for importing public keys from an x509 certificate
type X509PublicKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key importation algorithm identifier (to be used).
func (opts *X509PublicKeyImportOpts) Algorithm() string {
	fmt.Println("===X509PublicKeyImportOpts==Algorithm===")
	return X509Certificate
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *X509PublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===X509PublicKeyImportOpts==Ephemeral===")

	return opts.Temporary
}
