package util

import (
	"sync"
	"testing"

	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
)

type getLockTestCase struct {
	name string

	lockCreatedAfterRead bool
	getParallel          bool
	lockBefore           sync.Locker

	expectedLock sync.Locker
}

func TestGetLock(t *testing.T) {
	testCases := []getLockTestCase{
		getLockTestCase{
			name:        "Get existing lock",
			getParallel: true,
			lockBefore: &fakeLock{
				ID: 5,
			},
			expectedLock: &fakeLock{
				ID: 5,
			},
		},
		getLockTestCase{
			name:         "Get new lock",
			getParallel:  true,
			expectedLock: &sync.Mutex{},
		},
	}

	for _, testCase := range testCases {
		locks := map[string]sync.Locker{}
		if testCase.lockBefore != nil {
			locks["key"] = testCase.lockBefore
		}

		lockFactory := &defaultLockFactory{
			locks: locks,
		}

		parallelChan := make(chan sync.Locker)

		if testCase.getParallel {
			go func() {
				parallelChan <- lockFactory.GetLock("key")
			}()
		}

		lock := lockFactory.GetLock("key")

		lockAsYaml, err := yaml.Marshal(lock)
		assert.NilError(t, err, "Error parsing lock in testCase %s", testCase.name)
		expectedAsYaml, err := yaml.Marshal(testCase.expectedLock)
		assert.NilError(t, err, "Error parsing expectation in testCase %s", testCase.name)
		assert.Equal(t, string(lockAsYaml), string(expectedAsYaml), "Unexpected lock in testCase %s", testCase.name)

		if testCase.getParallel {
			parallelLock := <-parallelChan
			parallelAsYaml, err := yaml.Marshal(parallelLock)
			assert.NilError(t, err, "Error parsing parallel lock in testCase %s", testCase.name)
			assert.Equal(t, string(parallelAsYaml), string(expectedAsYaml), "Unexpected parallel lock in testCase %s", testCase.name)
		}
	}
}

type fakeLock struct {
	sync.Mutex
	ID int
}
