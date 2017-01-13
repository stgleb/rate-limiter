package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	Info  *log.Logger
	Error *log.Logger
	addr  string
	port  int
)

func init() {
	Info = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func AcquireToken(w http.ResponseWriter, r *http.Request) {

}

func CreateLimit(w http.ResponseWriter, r *http.Request) {

}

func main() {
	port = *flag.Int("port", 9000, "port number")
	addr = *flag.String("address", "0.0.0.0", "Address to listen")
	listenStr := fmt.Sprintf("%s:%d", addr, port)

	flag.Parse()

	http.HandleFunc("/aquire", AcquireToken)
	http.HandleFunc("/limits", CreateLimit)

	Info.Printf("Listen on %s", listenStr)
	if err := http.ListenAndServe(listenStr, nil); err != nil {
		Error.Fatal("ListenAndServe:", err)
	}
}
