/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lscc

import "fmt"

//InvalidFunctionErr invalid function error
type InvalidFunctionErr string

func (f InvalidFunctionErr) Error() string {
	fmt.Println("===InvalidFunctionErr==Error==")
	return fmt.Sprintf("invalid function to lscc: %s", string(f))
}

//InvalidArgsLenErr invalid arguments length error
type InvalidArgsLenErr int

func (i InvalidArgsLenErr) Error() string {
	fmt.Println("===InvalidArgsLenErr==Error==")
	return fmt.Sprintf("invalid number of arguments to lscc: %d", int(i))
}

//TXNotFoundErr transaction not found error
type TXNotFoundErr string

func (t TXNotFoundErr) Error() string {
	fmt.Println("===TXNotFoundErr==Error==")
	return fmt.Sprintf("transaction not found: %s", string(t))
}

//InvalidDeploymentSpecErr invalid chaincode deployment spec error
type InvalidDeploymentSpecErr string

func (f InvalidDeploymentSpecErr) Error() string {
	fmt.Println("===InvalidDeploymentSpecErr==Error==")
	return fmt.Sprintf("invalid deployment spec: %s", string(f))
}

//ExistsErr chaincode exists error
type ExistsErr string

func (t ExistsErr) Error() string {
	fmt.Println("===ExistsErr==Error==")
	return fmt.Sprintf("chaincode with name '%s' already exists", string(t))
}

//NotFoundErr chaincode not registered with LSCC error
type NotFoundErr string

func (t NotFoundErr) Error() string {
	fmt.Println("===NotFoundErr==Error==")
	return fmt.Sprintf("could not find chaincode with name '%s'", string(t))
}

//InvalidChannelNameErr invalid channel name error
type InvalidChannelNameErr string

func (f InvalidChannelNameErr) Error() string {
	fmt.Println("===InvalidChannelNameErr==Error==")
	return fmt.Sprintf("invalid channel name: %s", string(f))
}

//InvalidChaincodeNameErr invalid chaincode name error
type InvalidChaincodeNameErr string

func (f InvalidChaincodeNameErr) Error() string {
	fmt.Println("===InvalidChaincodeNameErr==Error==")
	return fmt.Sprintf("invalid chaincode name '%s'. Names can only consist of alphanumerics, '_', and '-'", string(f))
}

//EmptyChaincodeNameErr trying to upgrade to same version of Chaincode
type EmptyChaincodeNameErr string

func (f EmptyChaincodeNameErr) Error() string {
	fmt.Println("===EmptyChaincodeNameErr==Error==")
	return fmt.Sprint("chaincode name not provided")
}

//InvalidVersionErr invalid version error
type InvalidVersionErr string

func (f InvalidVersionErr) Error() string {
	fmt.Println("===InvalidVersionErr==Error==")
	return fmt.Sprintf("invalid chaincode version '%s'. Versions can only consist of alphanumerics, '_',  '-', '+', and '.'", string(f))
}

//InvalidStatedbArtifactsErr invalid state database artifacts error
type InvalidStatedbArtifactsErr string

func (f InvalidStatedbArtifactsErr) Error() string {
	fmt.Println("===InvalidStatedbArtifactsErr==Error==")
	return fmt.Sprintf("invalid state database artifact: %s", string(f))
}

//ChaincodeMismatchErr chaincode name from two places don't match
type ChaincodeMismatchErr string

func (f ChaincodeMismatchErr) Error() string {
	fmt.Println("===ChaincodeMismatchErr==Error==")
	return fmt.Sprintf("chaincode name mismatch: %s", string(f))
}

//EmptyVersionErr empty version error
type EmptyVersionErr string

func (f EmptyVersionErr) Error() string {
	fmt.Println("===EmptyVersionErr==Error==")
	return fmt.Sprintf("version not provided for chaincode with name '%s'", string(f))
}

//MarshallErr error marshaling/unmarshalling
type MarshallErr string

func (m MarshallErr) Error() string {
	fmt.Println("===MarshallErr==Error==")
	return fmt.Sprintf("error while marshalling: %s", string(m))
}

//IdenticalVersionErr trying to upgrade to same version of Chaincode
type IdenticalVersionErr string

func (f IdenticalVersionErr) Error() string {
	fmt.Println("===IdenticalVersionErr==Error==")
	return fmt.Sprintf("version already exists for chaincode with name '%s'", string(f))
}

//InvalidCCOnFSError error due to mismatch between fingerprint on lscc and installed CC
type InvalidCCOnFSError string

func (f InvalidCCOnFSError) Error() string {
	fmt.Println("===InvalidCCOnFSError==Error==")
	return fmt.Sprintf("chaincode fingerprint mismatch: %s", string(f))
}

//InstantiationPolicyMissing when no existing instantiation policy is found when upgrading CC
type InstantiationPolicyMissing string

func (f InstantiationPolicyMissing) Error() string {
	fmt.Println("===InstantiationPolicyMissing==Error==")
	return "instantiation policy missing"
}

// CollectionsConfigUpgradesNotAllowed when V1_2 capability is not enabled
type CollectionsConfigUpgradesNotAllowed string

func (f CollectionsConfigUpgradesNotAllowed) Error() string {
	fmt.Println("===CollectionsConfigUpgradesNotAllowed==Error==")
	return "as V1_2 capability is not enabled, collection upgrades are not allowed"
}

// PrivateChannelDataNotAvailable when V1_2 or later capability is not enabled
type PrivateChannelDataNotAvailable string

func (f PrivateChannelDataNotAvailable) Error() string {
	fmt.Println("===PrivateChannelDataNotAvailable==Error==")
	return "as V1_2 or later capability is not enabled, private channel collections and data are not available"
}
