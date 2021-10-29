/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bccsp

import (
	"crypto"
	"fmt"
)

// RevocationAlgorithm identifies the revocation algorithm
type RevocationAlgorithm int32

const (
	// IDEMIX constant to identify Idemix related algorithms
	IDEMIX = "IDEMIX"
)

const (
	// AlgNoRevocation means no revocation support
	AlgNoRevocation RevocationAlgorithm = iota
)

// IdemixIssuerKeyGenOpts contains the options for the Idemix Issuer key-generation.
// A list of attribytes may be optionally passed
type IdemixIssuerKeyGenOpts struct {
	// Temporary tells if the key is ephemeral
	Temporary bool
	// AttributeNames is a list of attributes
	AttributeNames []string
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixIssuerKeyGenOpts) Algorithm() string {
	fmt.Println("===IdemixIssuerKeyGenOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixIssuerKeyGenOpts) Ephemeral() bool {
	fmt.Println("===IdemixIssuerKeyGenOpts==Ephemeral===")
	return o.Temporary
}

// IdemixIssuerPublicKeyImportOpts contains the options for importing of an Idemix issuer public key.
type IdemixIssuerPublicKeyImportOpts struct {
	Temporary bool
	// AttributeNames is a list of attributes to ensure the import public key has
	AttributeNames []string
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixIssuerPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===IdemixIssuerPublicKeyImportOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixIssuerPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===IdemixIssuerPublicKeyImportOpts==Ephemeral===")
	return o.Temporary
}

// IdemixUserSecretKeyGenOpts contains the options for the generation of an Idemix credential secret key.
type IdemixUserSecretKeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixUserSecretKeyGenOpts) Algorithm() string {
	fmt.Println("===IdemixUserSecretKeyGenOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixUserSecretKeyGenOpts) Ephemeral() bool {
	fmt.Println("===IdemixUserSecretKeyGenOpts==Ephemeral===")
	return o.Temporary
}

// IdemixUserSecretKeyImportOpts contains the options for importing of an Idemix credential secret key.
type IdemixUserSecretKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixUserSecretKeyImportOpts) Algorithm() string {
	fmt.Println("===IdemixUserSecretKeyImportOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixUserSecretKeyImportOpts) Ephemeral() bool {
	fmt.Println("===IdemixUserSecretKeyImportOpts==Ephemeral===")
	return o.Temporary
}

// IdemixNymKeyDerivationOpts contains the options to create a new unlinkable pseudonym from a
// credential secret key with the respect to the specified issuer public key
type IdemixNymKeyDerivationOpts struct {
	// Temporary tells if the key is ephemeral
	Temporary bool
	// IssuerPK is the public-key of the issuer
	IssuerPK Key
}

// Algorithm returns the key derivation algorithm identifier (to be used).
func (*IdemixNymKeyDerivationOpts) Algorithm() string {
	fmt.Println("===IdemixNymKeyDerivationOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to derive has to be ephemeral,
// false otherwise.
func (o *IdemixNymKeyDerivationOpts) Ephemeral() bool {
	fmt.Println("===IdemixNymKeyDerivationOpts==Ephemeral===")
	return o.Temporary
}

// IssuerPublicKey returns the issuer public key used to derive
// a new unlinkable pseudonym from a credential secret key
func (o *IdemixNymKeyDerivationOpts) IssuerPublicKey() Key {
	fmt.Println("===IdemixNymKeyDerivationOpts==IssuerPublicKey===")
	return o.IssuerPK
}

// IdemixNymPublicKeyImportOpts contains the options to import the public part of a pseudonym
type IdemixNymPublicKeyImportOpts struct {
	// Temporary tells if the key is ephemeral
	Temporary bool
}

// Algorithm returns the key derivation algorithm identifier (to be used).
func (*IdemixNymPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===IdemixNymPublicKeyImportOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to derive has to be ephemeral,
// false otherwise.
func (o *IdemixNymPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===IdemixNymPublicKeyImportOpts==Ephemeral===")
	return o.Temporary
}

// IdemixCredentialRequestSignerOpts contains the option to create a Idemix credential request.
type IdemixCredentialRequestSignerOpts struct {
	// Attributes contains a list of indices of the attributes to be included in the
	// credential. The indices are with the respect to IdemixIssuerKeyGenOpts#AttributeNames.
	Attributes []int
	// IssuerPK is the public-key of the issuer
	IssuerPK Key
	// IssuerNonce is generated by the issuer and used by the client to generate the credential request.
	// Once the issuer gets the credential requests, it checks that the nonce is the same.
	IssuerNonce []byte
	// HashFun is the hash function to be used
	H crypto.Hash
}

func (o *IdemixCredentialRequestSignerOpts) HashFunc() crypto.Hash {
	fmt.Println("===IdemixCredentialRequestSignerOpts==HashFunc===")
	return o.H
}

// IssuerPublicKey returns the issuer public key used to derive
// a new unlinkable pseudonym from a credential secret key
func (o *IdemixCredentialRequestSignerOpts) IssuerPublicKey() Key {
	fmt.Println("===IdemixCredentialRequestSignerOpts==IssuerPublicKey===")
	return o.IssuerPK
}

// IdemixAttributeType represents the type of an idemix attribute
type IdemixAttributeType int

const (
	// IdemixHiddenAttribute represents an hidden attribute
	IdemixHiddenAttribute IdemixAttributeType = iota
	// IdemixStringAttribute represents a sequence of bytes
	IdemixBytesAttribute
	// IdemixIntAttribute represents an int
	IdemixIntAttribute
)

type IdemixAttribute struct {
	// Type is the attribute's type
	Type IdemixAttributeType
	// Value is the attribute's value
	Value interface{}
}

// IdemixCredentialSignerOpts contains the options to produce a credential starting from a credential request
type IdemixCredentialSignerOpts struct {
	// Attributes to include in the credentials. IdemixHiddenAttribute is not allowed here
	Attributes []IdemixAttribute
	// IssuerPK is the public-key of the issuer
	IssuerPK Key
	// HashFun is the hash function to be used
	H crypto.Hash
}

// HashFunc returns an identifier for the hash function used to produce
// the message passed to Signer.Sign, or else zero to indicate that no
// hashing was done.
func (o *IdemixCredentialSignerOpts) HashFunc() crypto.Hash {
	fmt.Println("===IdemixCredentialSignerOpts==HashFunc===")
	return o.H
}

func (o *IdemixCredentialSignerOpts) IssuerPublicKey() Key {
	fmt.Println("===IdemixCredentialSignerOpts==IssuerPublicKey===")
	return o.IssuerPK
}

// IdemixSignerOpts contains the options to generate an Idemix signature
type IdemixSignerOpts struct {
	// Nym is the pseudonym to be used
	Nym Key
	// IssuerPK is the public-key of the issuer
	IssuerPK Key
	// Credential is the byte representation of the credential signed by the issuer
	Credential []byte
	// Attributes specifies which attribute should be disclosed and which not.
	// If Attributes[i].Type = IdemixHiddenAttribute
	// then the i-th credential attribute should not be disclosed, otherwise the i-th
	// credential attribute will be disclosed.
	// At verification time, if the i-th attribute is disclosed (Attributes[i].Type != IdemixHiddenAttribute),
	// then Attributes[i].Value must be set accordingly.
	Attributes []IdemixAttribute
	// RhIndex is the index of attribute containing the revocation handler.
	// Notice that this attributed cannot be discloused
	RhIndex int
	// CRI contains the credential revocation information
	CRI []byte
	// Epoch is the revocation epoch the signature should be produced against
	Epoch int
	// RevocationPublicKey is the revocation public key
	RevocationPublicKey Key
	// H is the hash function to be used
	H crypto.Hash
}

func (o *IdemixSignerOpts) HashFunc() crypto.Hash {
	fmt.Println("===IdemixSignerOpts==HashFunc===")
	return o.H
}

// IdemixNymSignerOpts contains the options to generate an idemix pseudonym signature.
type IdemixNymSignerOpts struct {
	// Nym is the pseudonym to be used
	Nym Key
	// IssuerPK is the public-key of the issuer
	IssuerPK Key
	// H is the hash function to be used
	H crypto.Hash
}

// HashFunc returns an identifier for the hash function used to produce
// the message passed to Signer.Sign, or else zero to indicate that no
// hashing was done.
func (o *IdemixNymSignerOpts) HashFunc() crypto.Hash {
	fmt.Println("===IdemixNymSignerOpts==HashFunc===")
	return o.H
}

// IdemixRevocationKeyGenOpts contains the options for the Idemix revocation key-generation.
type IdemixRevocationKeyGenOpts struct {
	// Temporary tells if the key is ephemeral
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixRevocationKeyGenOpts) Algorithm() string {
	fmt.Println("===IdemixRevocationKeyGenOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixRevocationKeyGenOpts) Ephemeral() bool {
	fmt.Println("===IdemixRevocationKeyGenOpts==Ephemeral===")
	return o.Temporary
}

// IdemixRevocationPublicKeyImportOpts contains the options for importing of an Idemix revocation public key.
type IdemixRevocationPublicKeyImportOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (*IdemixRevocationPublicKeyImportOpts) Algorithm() string {
	fmt.Println("===IdemixRevocationPublicKeyImportOpts==Algorithm===")
	return IDEMIX
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (o *IdemixRevocationPublicKeyImportOpts) Ephemeral() bool {
	fmt.Println("===IdemixRevocationPublicKeyImportOpts==Ephemeral===")
	return o.Temporary
}

// IdemixCRISignerOpts contains the options to generate an Idemix CRI.
// The CRI is supposed to be generated by the Issuing authority and
// can be verified publicly by using the revocation public key.
type IdemixCRISignerOpts struct {
	Epoch               int
	RevocationAlgorithm RevocationAlgorithm
	UnrevokedHandles    [][]byte
	// H is the hash function to be used
	H crypto.Hash
}

func (o *IdemixCRISignerOpts) HashFunc() crypto.Hash {
	fmt.Println("===IdemixCRISignerOpts==HashFunc===")
	return o.H
}
