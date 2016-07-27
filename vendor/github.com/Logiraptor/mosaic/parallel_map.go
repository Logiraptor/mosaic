package main

import "sync"

func parallelMap(n int, f func(i int)) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			f(i)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func step(start, step int, inner func(int)) func(int) {
	return func(i int) {
		inner(start + i*step)
	}
}
