package main

import "time"

type Token struct{}

type Limit struct {
	Name      string
	Interval  int64
	Precision float64
	Count     int64
	Output    chan<- Token
	ShutDown  chan struct{}
}

func NewLimit(name string, interval, count int64, precision float64) *Limit {
	return &Limit{
		Name:      name,
		Interval:  interval,
		Count:     count,
		Precision: precision,
		Output:    make(chan<- Token),
		ShutDown:  make(chan struct{}),
	}
}

func (limit *Limit) Run() {
	ticker := time.NewTicker(time.Duration(limit.Interval / limit.Count))

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
