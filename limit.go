package main

import (
	"fmt"
	"time"
)

type Token struct{}

type Limit struct {
	Name      string
	Interval  int64
	Precision float64
	Count     int64
	Output    chan Token
	ShutDown  chan struct{}
}

// Create new limit, interval is set in milliseconds.
func NewLimit(name string, interval, count int64, precision float64) *Limit {
	limit := &Limit{
		Name:      name,
		Interval:  interval,
		Count:     count,
		Precision: precision,
		Output:    make(chan Token),
		ShutDown:  make(chan struct{}),
	}
	Info.Printf("Create limit %+v", *limit)

	return limit
}

func (limit *Limit) Run() {
	updateInterval := time.Duration(limit.Interval / limit.Count)
	ticker := time.NewTicker(time.Duration(updateInterval) * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			Info.Printf("Send new token")
			limit.Output <- Token{}
		case <-limit.ShutDown:
			Info.Printf("Channel %s shut down", limit.Name)
			return
		}
	}
}

func (limit *Limit) String() string {
	return fmt.Sprintf("<Name: %s, Interval: %d, Count: %d, Precision: %f>",
		limit.Name, limit.Interval, limit.Count, limit.Precision)
}
