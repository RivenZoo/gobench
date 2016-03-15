package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownCounter(t *testing.T) {
	total, bandwidth := 20, 5
	c := NewDownCounter(total, bandwidth)
	active := int32(0)
	count := int32(0)
	wg := sync.WaitGroup{}
	for i := 0; i < 2*bandwidth; i++ {
		wg.Add(1)
		ch := make(chan struct{})
		go func() {
			defer wg.Done()
			close(ch)
			for c.Borrow() {
				atomic.AddInt32(&count, 1)
				n := atomic.AddInt32(&active, 1)
				time.Sleep(time.Second)
				if n > int32(bandwidth) {
					t.FailNow()
				}
				t.Log(active)
				c.Payback()
				atomic.AddInt32(&active, -1)
			}
		}()
		<-ch
	}
	wg.Wait()
	if count > int32(total) {
		t.Fail()
	}
	t.Log(count)
}
