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

// ECDSAP256KeyGenOpts contains options for ECDSA key generation with curve P-256.
type ECDSAP256KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *ECDSAP256KeyGenOpts) Algorithm() string {
	fmt.Println("===ECDSAP256KeyGenOpts==Algorithm====")
	return ECDSAP256
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAP256KeyGenOpts) Ephemeral() bool {
	fmt.Println("===ECDSAP256KeyGenOpts==Ephemeral====")
	return opts.Temporary
}

// ECDSAP384KeyGenOpts contains options for ECDSA key generation with curve P-384.
type ECDSAP384KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *ECDSAP384KeyGenOpts) Algorithm() string {
	fmt.Println("===ECDSAP384KeyGenOpts==Algorithm====")
	return ECDSAP384
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *ECDSAP384KeyGenOpts) Ephemeral() bool {
	fmt.Println("===ECDSAP384KeyGenOpts==Ephemeral====")
	return opts.Temporary
}
