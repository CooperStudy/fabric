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

package util

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// Buffer provides a wrapper on top of proto.Buffer.
// The purpose of this wrapper is to get to know the current position in the []byte
type Buffer struct {
	buf      *proto.Buffer
	position int
}

// NewBuffer constructs a new instance of Buffer
func NewBuffer(b []byte) *Buffer {
	fmt.Println("====NewBuffer====")
	return &Buffer{proto.NewBuffer(b), 0}
}

// DecodeVarint wraps the actual method and updates the position
func (b *Buffer) DecodeVarint() (uint64, error) {
	fmt.Println("===Buffer=DecodeVarint====")
	val, err := b.buf.DecodeVarint()
	if err == nil {
		b.position += proto.SizeVarint(val)
	} else {
		err = errors.Wrap(err, "error decoding varint with proto.Buffer")
	}
	return val, err
}

// DecodeRawBytes wraps the actual method and updates the position
func (b *Buffer) DecodeRawBytes(alloc bool) ([]byte, error) {
	fmt.Println("===Buffer=DecodeRawBytes====")
	val, err := b.buf.DecodeRawBytes(alloc)
	if err == nil {
		b.position += proto.SizeVarint(uint64(len(val))) + len(val)
	} else {
		err = errors.Wrap(err, "error decoding raw bytes with proto.Buffer")
	}
	return val, err
}

// GetBytesConsumed returns the offset of the current position in the underlying []byte
func (b *Buffer) GetBytesConsumed() int {
	fmt.Println("===Buffer=GetBytesConsumed====")
	return b.position
}
