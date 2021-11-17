/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabenc

import (
	"fmt"
)

type Color uint8

const ColorNone Color = 0

const (
	ColorBlack Color = iota + 30
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

func (c Color) Normal() string {
	//fmt.Println("====Color==Normal====")
	return fmt.Sprintf("\x1b[%dm", c)
}

func (c Color) Bold() string {
	//fmt.Println("====Color==Bold====")
	if c == ColorNone {
		return c.Normal()
	}
	return fmt.Sprintf("\x1b[%d;1m", c)
}

func ResetColor() string {
	//fmt.Println("====ResetColor====")
	return ColorNone.Normal()
}
