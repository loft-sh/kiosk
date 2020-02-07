package util

import (
	"fmt"
	"sync"
	"testing"

	"gotest.tools/assert"
)

func TestGetLock(t *testing.T) {
	lockFactory := NewDefaultLockFactory()

	returnedLocksChan := make(chan []sync.Locker)
	for i := 0; i < 100; i++ {
		go func() {
			returnedLocks := make([]sync.Locker, 100)
			for i := 0; i < 100; i++ {
				returnedLocks[i] = lockFactory.GetLock(fmt.Sprintf("key%d", i))
			}
			returnedLocksChan <- returnedLocks
		}()
	}

	returnedLocks := make([][]sync.Locker, 100)
	for i := 0; i < 100; i++ {
		returnedLocks[i] = <-returnedLocksChan
	}

	for i := 1; i < 100; i++ {
		for j := 0; j < 100; j++ {
			assert.Equal(t, returnedLocks[0][j], returnedLocks[i][j], "Unequal locks for index %d/%d", i, j)
		}
	}
}
