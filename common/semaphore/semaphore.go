/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package semaphore

import (
	"context"
	"fmt"
)

type Semaphore chan struct{}

func New(count int) Semaphore {
	fmt.Println("===New===")
	if count <= 0 {
		panic("count must be greater than 0")
	}
	return make(chan struct{}, count)
}

func (s Semaphore) Acquire(ctx context.Context) error {
	fmt.Println("===Semaphore==Acquire=")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s <- struct{}{}:
		return nil
	}
}

func (s Semaphore) Release() {
	fmt.Println("===Semaphore==Release=")
	select {
	case <-s:
	default:
		panic("semaphore buffer is empty")
	}
}
