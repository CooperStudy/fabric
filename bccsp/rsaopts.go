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

// RSA1024KeyGenOpts contains options for RSA key generation at 1024 security.
type RSA1024KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *RSA1024KeyGenOpts) Algorithm() string {
	fmt.Println("===RSA1024KeyGenOpts==Algorithm===")
	return RSA1024
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSA1024KeyGenOpts) Ephemeral() bool {
	fmt.Println("===RSA1024KeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// RSA2048KeyGenOpts contains options for RSA key generation at 2048 security.
type RSA2048KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *RSA2048KeyGenOpts) Algorithm() string {
	fmt.Println("===RSA2048KeyGenOpts==Algorithm===")
	return RSA2048
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSA2048KeyGenOpts) Ephemeral() bool {
	fmt.Println("===RSA2048KeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// RSA3072KeyGenOpts contains options for RSA key generation at 3072 security.
type RSA3072KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *RSA3072KeyGenOpts) Algorithm() string {
	fmt.Println("===RSA3072KeyGenOpts==Algorithm===")
	return RSA3072
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSA3072KeyGenOpts) Ephemeral() bool {
	fmt.Println("===RSA3072KeyGenOpts==Ephemeral===")
	return opts.Temporary
}

// RSA4096KeyGenOpts contains options for RSA key generation at 4096 security.
type RSA4096KeyGenOpts struct {
	Temporary bool
}

// Algorithm returns the key generation algorithm identifier (to be used).
func (opts *RSA4096KeyGenOpts) Algorithm() string {
	fmt.Println("===RSA4096KeyGenOpts==Algorithm===")
	return RSA4096
}

// Ephemeral returns true if the key to generate has to be ephemeral,
// false otherwise.
func (opts *RSA4096KeyGenOpts) Ephemeral() bool {
	fmt.Println("===RSA4096KeyGenOpts==Ephemeral===")
	return opts.Temporary
}
