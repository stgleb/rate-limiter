package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	Info    *log.Logger
	Error   *log.Logger
	limit   *Limit
	Host    string
	timeout int64
	port    int
	// TODO(gstepanov): add concurrent hash map instead of native.
	limitsMap map[string]Limit
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

}

func UpdateLimit(w http.ResponseWriter, r *http.Request) {

}

func main() {
	port = *flag.Int("port", 9000, "port number")
	Host = *flag.String("address", "0.0.0.0", "Address to listen")
	listenStr := fmt.Sprintf("%s:%d", Host, port)

	// Create and start new limit
	// Allows to get one token per 3 second.
	limit = NewLimit("limit0", 1000*3, 1, 0.1)
	go limit.Run()
	flag.Parse()
	router := mux.NewRouter()

	router.HandleFunc("/limit/{limit}/acquire", AcquireToken).Methods(http.MethodHead)
	router.HandleFunc("/limit", CreateLimit).Methods(http.MethodPost)
	router.HandleFunc("/limit", UpdateLimit).Methods(http.MethodPut)

	Info.Printf("Listen on %s", listenStr)
	if err := http.ListenAndServe(listenStr, router); err != nil {
		Error.Fatal("ListenAndServe:", err)
	}
}
