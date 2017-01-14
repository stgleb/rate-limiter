package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestLimitAcqireToken(t *testing.T) {
	limit = NewLimit("limit0", 1, 1, 0.1)
	go limit.Run()
	defer func() {
		limit.ShutDown <- struct{}{}
	}()

	token := <-limit.Output

	switch token.(type) {
	case Token:
	default:
		t.Errorf("Unknown type %t of token", token)
	}
}

func TestLimitFrequency(t *testing.T) {
	interval := 100
	count := 5
	limit = NewLimit("limit0", interval, count, 0.1)
	tick := time.NewTicker(time.Duration(interval) * time.Millisecond)
	go limit.Run()

	for {
		select {
		case <-limit.Output:
			count--
		case <-tick.C:
			// Use unconditional because break doesn't break encompassing for loop
			goto exit
		}
	}

exit:
	assert.Equal(t, count, 0)
}

func TestLimitUpdate(t *testing.T) {

}

func TestLimitGetConf(t *testing.T) {

}
