package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	limiter "github.com/adeelkhan/rlimiter/ratelimiter"
	"github.com/gorilla/mux"
)

type Failure struct {
	Msg    string `json:"Msg"`
	Status int    `json:"Status"`
}

var rl = limiter.NewRateLimiter()

func testapi(w http.ResponseWriter, r *http.Request) {
	if ok := rl.Check(r); !ok {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "testapi\n")
	} else {
		json, err := json.Marshal(Failure{
			Msg:    "Blocked too many calls",
			Status: http.StatusTooManyRequests,
		})
		if err != nil {
			fmt.Println(err)
		}
		fmt.Fprint(w, string(json))
	}
}

func main() {
	// setup router plus handler
	fmt.Println("Running server")
	r := mux.NewRouter()
	r.HandleFunc("/", testapi)
	srv := &http.Server{
		Handler:      r,
		Addr:         "localhost:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
