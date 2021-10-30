/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package errors

import "fmt"

// TxValidationError marks that the error is related to
// validation of a transaction
type TxValidationError interface {
	error
	IsValid() bool
}

// VSCCInfoLookupFailureError error to indicate inability
// to obtain VSCC information from LCCC
type VSCCInfoLookupFailureError struct {
	Reason string
}

// Error returns reasons which lead to the failure
func (e VSCCInfoLookupFailureError) Error() string {
	fmt.Println("======VSCCInfoLookupFailureError==Error=")
	return e.Reason
}

// VSCCEndorsementPolicyError error to mark transaction
// failed endorsement policy check
type VSCCEndorsementPolicyError struct {
	Err error
}

func (e *VSCCEndorsementPolicyError) IsValid() bool {
	fmt.Println("======VSCCEndorsementPolicyError==IsValid=")
	return e.Err == nil
}

// Error returns reasons which lead to the failure
func (e VSCCEndorsementPolicyError) Error() string {
	fmt.Println("======VSCCEndorsementPolicyError==Error=")
	return e.Err.Error()
}

// VSCCExecutionFailureError error to indicate
// failure during attempt of executing VSCC
// endorsement policy check
type VSCCExecutionFailureError struct {
	Err error
}

// Error returns reasons which lead to the failure
func (e VSCCExecutionFailureError) Error() string {
	fmt.Println("======VSCCExecutionFailureError==Error=")
	return e.Err.Error()
}

func (e *VSCCExecutionFailureError) IsValid() bool {
	fmt.Println("======VSCCExecutionFailureError===IsValid==")
	return e.Err == nil
}
