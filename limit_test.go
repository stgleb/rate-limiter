package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func TestLimitAcqireToken(t *testing.T) {
	Info.Printf("TestLimitAcqireToken")
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
	Info.Printf("TestLimitFrequency")
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
	Info.Printf("TestLimitUpdate")
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
	Info.Printf("TestLimitGetConf")
	limit := NewLimit("test", 10, 4, 0.1)
	go limit.Run()
	limitConf := <-limit.GetConf
	assert.Equal(t, limit.Name, limitConf.Name)
	assert.Equal(t, limit.Count, limitConf.Count)
	assert.Equal(t, limit.Interval, limitConf.Interval)
}

func TestGetLimitHttp(t *testing.T) {
	Info.Printf("TestGetLimitHttp")
	limitServer := LimitServer{
		limitsMap: make(map[string]Limit),
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(limitServer.GetLimit)
	req := httptest.NewRequest(http.MethodGet, "/limit/1", nil)
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestCreateLimitHttp201(t *testing.T) {
	Info.Printf("TestCreateLimitHttp201")
	limitServer := LimitServer{
		limitsMap: make(map[string]Limit),
	}
	router := mux.NewRouter()
	router.HandleFunc("/limit", limitServer.CreateLimit)
	limit := NewLimit("foo", 1, 1, 0.1)
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(limit)

	if err != nil {
		t.Errorf("Error while encoding limit to json %s", err.Error())
	}

	port := 1024 + (rand.Int() & (1 << 15))

	// Run http server
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	}()

	url := fmt.Sprintf("http://0.0.0.0:%d/limit", port)
	req, _ := http.NewRequest(http.MethodPost, url, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("Error during request %s", err.Error())
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Wrong response code actual %d expected %d",
			resp.StatusCode, http.StatusCreated)
	}
}

func TestCreateLimitHttp409(t *testing.T) {
	Info.Printf("TestCreateLimitHttp409")
	limit := NewLimit("foo", 1, 1, 0.1)
	limitsMap := map[string]Limit{
		"foo": *limit,
	}
	limitServer := LimitServer{
		limitsMap: limitsMap,
	}
	router := mux.NewRouter()
	router.HandleFunc("/limit", limitServer.CreateLimit)
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(limit)

	if err != nil {
		t.Errorf("Error while encoding limit to json %s", err.Error())
	}

	port := 1024 + (rand.Int() & (1 << 15))

	// Run http server
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	}()

	url := fmt.Sprintf("http://0.0.0.0:%d/limit", port)
	req, _ := http.NewRequest(http.MethodPost, url, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("Error during request %s", err.Error())
	}

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Wrong response code actual %d expected %d",
			resp.StatusCode, http.StatusConflict)
	}
}

func TestUpdateLimitHttp(t *testing.T) {

}

func TestDeleteLimitHttp(t *testing.T) {

}
