package main

import "sync/atomic"

type DownCounter struct {
	total     int32
	bandwidth chan bool
}

func NewDownCounter(total, bandwidth int) *DownCounter {
	c := &DownCounter{
		int32(total),
		make(chan bool, bandwidth),
	}
	for i := 0; i < bandwidth; i++ {
		c.bandwidth <- true
	}
	return c
}

// if return false means all count consumed
func (c *DownCounter) Borrow() bool {
	old := atomic.AddInt32(&c.total, -1)
	if old < 0 {
		return false
	}
	<- c.bandwidth
	return true
}

func (c *DownCounter) Payback() {
	c.bandwidth <- true
}
