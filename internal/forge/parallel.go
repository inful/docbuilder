package forge

import "sync"

type orderedResult[T any] struct {
	Value T
	Err   error
}

func runOrdered[T any, R any](items []T, concurrency int, fn func(T) (R, error)) []orderedResult[R] {
	if len(items) == 0 {
		return nil
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(items) {
		concurrency = len(items)
	}

	sem := make(chan struct{}, concurrency)
	results := make([]orderedResult[R], len(items))

	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(i int, item T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			v, err := fn(item)
			results[i] = orderedResult[R]{Value: v, Err: err}
		}(i, item)
	}
	wg.Wait()
	return results
}
