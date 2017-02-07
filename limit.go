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
	LimitId   string         `json:"limit_id"`
	OfferId   string         `json:"offer_id"`
	Name      string         `json:"name"`
	Interval  int            `json:"interval"`
	Precision float64        `json:"precision"`
	Count     int            `json:"count"`
	IsDeleted bool           `json:"-"`
	UpdatedAt int64          `json:"count"`
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

// Create new limit, interval is set in milliseconds.
func NewEmptyLimit() *Limit {
	limit := &Limit{
		Name:      "",
		Interval:  0,
		Count:     0,
		Precision: 0.0,
		Output:    make(chan Token),
		ShutDown:  make(chan struct{}),
		Update:    make(chan LimitConf),
		GetConf:   make(chan LimitConf),
	}
	Info.Printf("Create empty limit %+v", *limit)

	return limit
}

func (limit *Limit) Run() {
	updateInterval := time.Duration(limit.Interval * 1000 * 1000 * 1000 / limit.Count)
	Info.Printf("Limit %s Update interval %v", limit.LimitId, updateInterval)
	ticker := time.NewTicker(updateInterval)

	currentConf := LimitConf{
		Name:      limit.Name,
		Count:     limit.Count,
		Interval:  limit.Interval,
		Precision: limit.Precision,
	}

	for {
		select {
		case <-ticker.C:
			Info.Printf("Limit %s Send new token", limit.LimitId)
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
			updateInterval = time.Duration(limit.Interval * 1000 * 1000 / limit.Count)
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
	return fmt.Sprintf("<Name: %s, OfferId: %s, Interval: %d, Count: %d, Precision: %f, UpdatedAt: %d>",
		limit.Name, limit.OfferId, limit.Interval, limit.Count, limit.Precision, limit.UpdatedAt)
}
