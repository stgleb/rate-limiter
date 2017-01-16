package main

import (
	"fmt"
	"github.com/pkg/errors"
	"time"
)

type Token struct{}

type LimitConf struct {
	Name      string  `json:"name"`
	Interval  int     `json:"interval"`
	Count     int     `json:"count"`
	Precision float64 `json:"precision"`
}

type Limit struct {
	Name      string         `json:"name"`
	Interval  int            `json:"interval"`
	Precision float64        `json:"precision"`
	Count     int            `json:"count"`
	Output    chan Token     `json:"-"`
	ShutDown  chan struct{}  `json:"-"`
	Update    chan LimitConf `json:"-"`
	GetConf   chan LimitConf `json:"-"`
}

func NewLimitConfig(name string, interval, count int, precision float64) (LimitConf, error) {
	if count < 0 {
		return LimitConf{}, errors.New("Count cannot be negative")
	}

	if interval < 0 {
		return LimitConf{}, errors.New("Interval cannot be negative")
	}

	conf := LimitConf{
		Name:      name,
		Interval:  interval,
		Count:     count,
		Precision: precision,
	}
	Info.Printf("Create limit config %+v", conf)

	return conf, nil
}

// Create new limit, interval is set in milliseconds.
func NewLimit(name string, interval, count int, precision float64) *Limit {
	limit := &Limit{
		Name:      name,
		Interval:  interval,
		Count:     count,
		Precision: precision,
		Output:    make(chan Token),
		ShutDown:  make(chan struct{}),
		Update:    make(chan LimitConf),
		GetConf:   make(chan LimitConf),
	}
	Info.Printf("Create limit %+v", *limit)

	return limit
}

func (limit *Limit) Run() {
	updateInterval := time.Duration(limit.Interval / limit.Count)
	ticker := time.NewTicker(time.Duration(updateInterval) * time.Millisecond)
	currentConf := LimitConf{
		Name:      limit.Name,
		Count:     limit.Count,
		Interval:  limit.Interval,
		Precision: limit.Precision,
	}

	for {
		select {
		case <-ticker.C:
			Info.Printf("Send new token")
			limit.Output <- Token{}
		case <-limit.ShutDown:
			Info.Printf("Channel %s shut down", limit.Name)
			return
		case currentConf = <-limit.Update:
			Info.Printf("Update limit to %v", currentConf)
			// TODO(stgleb): consider check if values were changed
			limit.Count = currentConf.Count
			limit.Interval = currentConf.Interval
			limit.Precision = currentConf.Precision
			// TODO(stgleb): consider whether manipulation of ticker inside select is racy.
			updateInterval := time.Duration(limit.Interval / limit.Count)
			ticker = time.NewTicker(time.Duration(updateInterval) * time.Millisecond)

			if len(currentConf.Name) > 0 {
				limit.Name = currentConf.Name
			}
		case limit.GetConf <- currentConf:
			// Try to always send current configuration to external
			// consumer.
		}
	}
}

func (limit Limit) String() string {
	return fmt.Sprintf("<Name: %s, Interval: %d, Count: %d, Precision: %f>",
		limit.Name, limit.Interval, limit.Count, limit.Precision)
}
