package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	Info    *log.Logger
	Error   *log.Logger
	Host    string
	timeout int64
	port    int
	// TODO(stgleb): extract all data to struct LimtiServer to make code better testable.
	limitsMap map[string]Limit
	lock      sync.RWMutex
)

func init() {
	limitsMap = make(map[string]Limit)

	Info = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func AcquireToken(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	lock.RLock()
	limit, ok := limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	lock.RUnlock()
	t := time.After(time.Duration(timeout))

	// Acquire token from limit.
	select {
	case <-limit.Output:
		w.WriteHeader(http.StatusOK)
	case <-t:
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

func CreateLimit(w http.ResponseWriter, r *http.Request) {
	var limit Limit

	if err := json.NewDecoder(r.Body).Decode(&limit); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get limit from map in thread safe way
	lock.Lock()
	_, ok := limitsMap[limit.Name]

	if ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	limitsMap[limit.Name] = limit
	w.WriteHeader(http.StatusCreated)
	lock.Unlock()
}

func GetLimit(w http.ResponseWriter, r *http.Request) {
	var limitConf LimitConf
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	lock.RLock()
	limit, ok := limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	limitConf = <-limit.GetConf
	lock.RUnlock()
	json.NewEncoder(w).Encode(limitConf)
}

func UpdateLimit(w http.ResponseWriter, r *http.Request) {
	var limitConf LimitConf

	if err := json.NewDecoder(r.Body).Decode(limitConf); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get limit from map in thread safe way
	lock.RLock()
	limit, ok := limitsMap[limitConf.Name]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		limit.Update <- limitConf
	}
	lock.RUnlock()
	w.WriteHeader(http.StatusAccepted)
}

func DeleteLimit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	lock.Lock()
	limit, ok := limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(limitsMap, limitName)
	limit.ShutDown <- struct{}{}
	lock.Unlock()
}

func main() {
	port = *flag.Int("port", 9000, "port number")
	Host = *flag.String("address", "0.0.0.0", "Address to listen")
	listenStr := fmt.Sprintf("%s:%d", Host, port)

	flag.Parse()
	router := mux.NewRouter()

	router.HandleFunc("/limit/{limit}/acquire", AcquireToken).Methods(http.MethodHead)
	router.HandleFunc("/limit/{limit}", GetLimit).Methods(http.MethodGet)
	router.HandleFunc("/limit", CreateLimit).Methods(http.MethodPost)
	router.HandleFunc("/limit", UpdateLimit).Methods(http.MethodPut)
	router.HandleFunc("/limit/{limit}", DeleteLimit).Methods(http.MethodDelete)

	Info.Printf("Listen on %s", listenStr)
	if err := http.ListenAndServe(listenStr, router); err != nil {
		Error.Fatal("ListenAndServe:", err)
	}
}
