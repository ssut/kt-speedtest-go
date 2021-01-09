package main

import (
	"context"
	"sync/atomic"
)

type TrafficCounter struct {
	Total    *int64
	Interval *int64

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTrafficCounter() TrafficCounter {
	var total int64
	var interval int64
	return TrafficCounter{
		Total:    &total,
		Interval: &interval,
	}
}

func (tc *TrafficCounter) GetTotal() int64 {
	return atomic.LoadInt64(tc.Total)
}

func (tc *TrafficCounter) GetInterval() int64 {
	val := atomic.LoadInt64(tc.Interval)
	atomic.StoreInt64(tc.Interval, 0)

	return val
}

func (tc *TrafficCounter) AddCount(value int64) {
	atomic.AddInt64(tc.Total, value)
	atomic.AddInt64(tc.Interval, value)
}

func (tc *TrafficCounter) Run() (chan<- int64, context.CancelFunc) {
	tc.ctx, tc.cancel = context.WithCancel(context.Background())
	ch := make(chan int64, 1024)

	go func() {
		for {
			select {
			case n := <-ch:
				tc.AddCount(int64(n))
				break

			case <-tc.ctx.Done():
				return
			}
		}
	}()

	return ch, tc.cancel
}
