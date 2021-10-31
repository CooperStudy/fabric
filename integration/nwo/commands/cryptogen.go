/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commands

import "fmt"

type Generate struct {
	Config string
	Output string
}

func (c Generate) SessionName() string {
	fmt.Println("=============Generate=====SessionName==============")
	return "cryptogen-generate"
}

func (c Generate) Args() []string {
	fmt.Println("=============Generate=====Args==============")
	return []string{
		"generate",
		"--config", c.Config,
		"--output", c.Output,
	}
}
