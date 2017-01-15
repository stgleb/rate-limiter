package main

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimitAcqireToken(t *testing.T) {
	limit := NewLimit("limit0", 1, 1, 0.1)
	go limit.Run()
	defer func() {
		limit.ShutDown <- struct{}{}
	}()

	select {
	case <-limit.Output:
	case <-time.After(time.Millisecond * 2):
	}
}

func TestLimitFrequency(t *testing.T) {
	interval := 100
	count := 5
	limit := NewLimit("limit0", interval, count, 0.1)
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
	if count < 0 {
		t.Errorf("Limit exceed by %d", -count)
	}
}

func TestLimitUpdate(t *testing.T) {
	limit := NewLimit("test", 1, 1, 0.1)
	limitConfig, err := NewLimitConfig("", 2, 2, 0.1)

	if err != nil {
		t.Errorf("Error creating limit config %s", err.Error())
	}

	go limit.Run()
	limit.Update <- limitConfig
	assert.Equal(t, limit.Count, limitConfig.Count)
	assert.Equal(t, limit.Interval, limitConfig.Interval)
}

func TestLimitGetConf(t *testing.T) {
	limit := NewLimit("test", 10, 4, 0.1)
	go limit.Run()
	limitConf := <-limit.GetConf
	assert.Equal(t, limit.Name, limitConf.Name)
	assert.Equal(t, limit.Count, limitConf.Count)
	assert.Equal(t, limit.Interval, limitConf.Interval)
}

func TestGetLimitHttp(t *testing.T) {
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetLimit)
	req := httptest.NewRequest(http.MethodGet, "/limit/1", nil)
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestCreateLimitHttp(t *testing.T) {

}

func TestUpdateLimitHttp(t *testing.T) {

}

func TestDeleteLimitHttp(t *testing.T) {

}
