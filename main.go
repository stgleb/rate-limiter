package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"
)

var (
	Info         *log.Logger
	Error        *log.Logger
	Host         string
	timeout      int64
	port         int
	pprofEnabled bool
	pprofport    int
	db           *sql.DB
)

type LimitServer struct {
	limitsMap map[string]Limit
	sync.RWMutex
}

func LoadLimits() []*Limit {
	var Id int
	limits := make([]*Limit, 0, 32)

	tx, err := db.Begin()
	if err != nil {
		Error.Print(err)
	}
	defer tx.Rollback()

	rows, err := tx.Query("SELECT * FROM limits where is_deleted = 0")
	defer rows.Close()

	if err != nil {
		Error.Print(err)
	}

	for rows.Next() {
		limit := NewEmptyLimit()
		err := rows.Scan(&Id, &limit.OfferId, &limit.LimitId,
			&limit.Count, &limit.Interval, &limit.Precision,
			&limit.IsDeleted, &limit.UpdatedAt)
		if err != nil {
			log.Fatal(err)
		}
		// Collect limits
		limits = append(limits, limit)
	}
	err = rows.Err()

	if err != nil {
		Error.Print(err)
	}

	err = tx.Commit()
	if err != nil {
		Error.Print(err)
	}

	return limits
}

func InitDb() {
	// NOTE: err is declared in advance not to omit db
	var err error
	db, err = sql.Open("sqlite3", "./limits.db")

	if err != nil {
		Error.Print("Cannot access databse")
	}

	if err = db.Ping(); err != nil {
		Error.Print("Cannot ping database")
	}

	// Create table if not exists
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS limits ( id integer PRIMARY KEY, offer_id varchar(255) NOT NULL, limit_id varchar(255) NOT NULL, count INTEGER(255), interval INTEGER(255), precision real, is_deleted INTEGER(1), updated_at INTEGER(32))")

	if err != nil {
		Error.Print(err)
	}
}

func init() {
	flag.IntVar(&port, "port", 9000, "port number")
	flag.StringVar(&Host, "address", "0.0.0.0", "Address to listen")
	flag.BoolVar(&pprofEnabled, "pprof", false, "enable pprof for profiling")
	flag.IntVar(&pprofport, "pprofPort", 6060, "pprof port")
	flag.Parse()

	Info = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	// Initialize database
	InitDb()
}

func (limitServer *LimitServer) AcquireToken(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	limitServer.RLock()
	limit, ok := limitServer.limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	limitServer.RUnlock()
	t := time.After(time.Duration(timeout))

	// Acquire token from limit.
	select {
	case <-limit.Output:
		w.WriteHeader(http.StatusOK)
	case <-t:
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

func (limitServer *LimitServer) CreateLimit(w http.ResponseWriter, r *http.Request) {
	var limit Limit

	if err := json.NewDecoder(r.Body).Decode(&limit); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get limit from map in thread safe way
	limitServer.Lock()
	_, ok := limitServer.limitsMap[limit.Name]

	if ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	limitServer.limitsMap[limit.Name] = limit
	w.WriteHeader(http.StatusCreated)
	limitServer.Unlock()
}

func (limitServer *LimitServer) GetLimit(w http.ResponseWriter, r *http.Request) {
	var limitConf LimitConf
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	limitServer.RLock()
	limit, ok := limitServer.limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	limitConf = <-limit.GetConf
	limitServer.RUnlock()
	json.NewEncoder(w).Encode(limitConf)
}

func (limitServer *LimitServer) UpdateLimit(w http.ResponseWriter, r *http.Request) {
	var limitConf LimitConf

	if err := json.NewDecoder(r.Body).Decode(&limitConf); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get limit from map in thread safe way
	limitServer.RLock()
	limit, ok := limitServer.limitsMap[limitConf.Name]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		limit.Update <- limitConf
	}
	limitServer.RUnlock()
	w.WriteHeader(http.StatusAccepted)
}

func (limitServer *LimitServer) DeleteLimit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	limitName := vars["limit"]

	// Get limit from map in thread safe way
	limitServer.Lock()
	limit, ok := limitServer.limitsMap[limitName]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(limitServer.limitsMap, limitName)
	limit.ShutDown <- struct{}{}
	limitServer.Unlock()
}

func (limitServer *LimitServer) SubstituteLimits() {
	Info.Print("Substitute limits")
	// Load limits from database
	limits := LoadLimits()
	limitServer.Lock()
	defer limitServer.Unlock()
	// Substitute old limits by new, old limits are shut down
	for index := range limits {
		limit := limits[index]
		oldLimit, ok := limitServer.limitsMap[limit.LimitId]

		if ok && limit.UpdatedAt != oldLimit.UpdatedAt {
			Info.Printf("Update limit %s", limit.LimitId)

			// Send stop signal to all limit goroutines
			close(oldLimit.ShutDown)

			// Save limit to map and start it
			limitServer.limitsMap[limit.LimitId] = *limit
			go limit.Run()
		} else if !ok {
			// If limit is new
			Info.Printf("Start limit %s", limit.LimitId)
			limitServer.limitsMap[limit.LimitId] = *limit
			go limit.Run()
		}
	}
}

func (limitServer *LimitServer) LimitUpdater() {
	ticker := time.NewTicker(time.Duration(5) * time.Second)

	for {
		select {
		case <-ticker.C:
			Info.Print("Update limits")
			limitServer.SubstituteLimits()
		}
	}
}

func main() {
	if pprofEnabled {
		pprofUrl := fmt.Sprintf("localhost:%d", pprofport)
		Info.Printf("Start profiling on %s", pprofUrl)
		go func() {
			log.Println(http.ListenAndServe(pprofUrl, nil))
		}()
	}

	listenStr := fmt.Sprintf("%s:%d", Host, port)
	limitServer := LimitServer{
		limitsMap: make(map[string]Limit),
	}
	limitServer.SubstituteLimits()
	go limitServer.LimitUpdater()

	router := mux.NewRouter()

	router.HandleFunc("/limit/{limit}/acquire", limitServer.AcquireToken).Methods(http.MethodGet)
	router.HandleFunc("/limit/{limit}", limitServer.GetLimit).Methods(http.MethodGet)
	router.HandleFunc("/limit", limitServer.CreateLimit).Methods(http.MethodPost)
	router.HandleFunc("/limit", limitServer.UpdateLimit).Methods(http.MethodPut)
	router.HandleFunc("/limit/{limit}", limitServer.DeleteLimit).Methods(http.MethodDelete)

	Info.Printf("Listen on %s", listenStr)
	if err := http.ListenAndServe(listenStr, router); err != nil {
		Error.Fatal("ListenAndServe:", err)
	}
}
