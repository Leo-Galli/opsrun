// SPDX-License-Identifier: Apache-2.0

package ssh

import "context"

// semaphore bounds the number of concurrent holders of a resource.
type semaphore chan struct{}

func newSemaphore(n int) semaphore {
	return make(semaphore, n)
}

// Acquire blocks until a slot is free or ctx is done.
func (s semaphore) Acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a previously acquired slot.
func (s semaphore) Release() {
	<-s
}
