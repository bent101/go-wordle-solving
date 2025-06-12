package main

import (
	"sync"

	"golang.org/x/exp/constraints"
)

// MinBy finds the minimum element using a key function (like lodash's minBy)
func MinBy[T any, K constraints.Ordered](slice []T, keyFunc func(T) K) T {
	if len(slice) == 0 {
		var zero T
		return zero
	}

	minKey := slice[0]
	minVal := keyFunc(minKey)

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for _, key := range slice {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val := keyFunc(key)
			mu.Lock()
			if val < minVal {
				minVal = val
				minKey = key
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	return minKey
}
