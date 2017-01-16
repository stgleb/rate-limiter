package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
"github.com/gorilla/mux"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// Ask the kernel for a free open port that is ready to use
func GetPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
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
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(limitServer.CreateLimit)
	limit := NewLimit("foo", 1, 1, 0.1)
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(limit)

	if err != nil {
		t.Errorf("Error while encoding limit to json %s", err.Error())
	}

	req := httptest.NewRequest(http.MethodPost, "/limit", body)
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
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
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(limitServer.CreateLimit)
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(limit)

	if err != nil {
		t.Errorf("Error while encoding limit to json %s", err.Error())
	}

	req := httptest.NewRequest(http.MethodPost, "/limit", body)
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusConflict)
	}
}

func TestUpdateLimitHttp202(t *testing.T) {
	Info.Printf("TestUpdateLimitHttp202")
	limit := NewLimit("foo", 1, 1, 0.1)
	// Run limit to be able to receive config updates
	go limit.Run()
	// Shut down limit in the end
	defer func() {
		limit.ShutDown <- struct{}{}
	}()

	limitConf := LimitConf{
		Name:      "foo",
		Count:     2,
		Interval:  2,
		Precision: 0.1,
	}
	limitsMap := map[string]Limit{
		"foo": *limit,
	}
	limitServer := LimitServer{
		limitsMap: limitsMap,
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(limitServer.UpdateLimit)

	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(limitConf)

	if err != nil {
		t.Errorf("Error while encoding limitconf to json %s", err.Error())
	}

	req := httptest.NewRequest(http.MethodPut, "/limit", body)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusAccepted)
	}
}

func TestDeleteLimitHttp200(t *testing.T) {
	Info.Printf("TestDeleteLimitHttp200")
	limit := NewLimit("foo", 1, 1, 0.1)
	// Run limit to be able to receive shutdown signal
	go limit.Run()
	// Run limit to be able to receive config updates
	limitName := "foo"

	limitServer := LimitServer{
		limitsMap: map[string]Limit{
			limitName: *limit,
		},
	}
	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/limit/{limit}", limitServer.DeleteLimit).Methods(http.MethodDelete)

	url := fmt.Sprintf("/limit/%s", limitName)
	req := httptest.NewRequest(http.MethodDelete, url, nil)
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
