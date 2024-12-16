package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

const REQUESTS = 10
const TIME_WINDOW = 20

// stringify the rate limit config
// const API_RATELIMIT = "20/min"

// add block time
//const BLOCK_TIME = 60 //sec

type Failure struct {
	Msg    string `json:"Msg"`
	Status int    `json:"Status"`
}

type RateLimiter struct {
	client *redis.Client
}

var rl = newRateLimiter()

func newRateLimiter() RateLimiter {
	return RateLimiter{
		client: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		}),
	}
}
func (rl *RateLimiter) algo1(ipAddr string) bool {
	ctx := context.Background()
	res2, err := rl.client.HGetAll(ctx, ipAddr).Result()
	if err != nil {
		panic(err)
	}
	if len(res2) == 0 {
		data := map[string]interface{}{
			"timeStamp": time.Now().Unix(),
			"count":     "0",
		}
		_, err := rl.client.HSet(ctx, ipAddr, data).Result()
		if err != nil {
			panic(err)
		}
		log.Print("IpAddr not found: saving", data)
	} else {
		now := time.Now().Unix()
		firstTimeStamp, _ := strconv.ParseInt(res2["timeStamp"], 10, 64)
		diff := now - firstTimeStamp
		counter, _ := strconv.Atoi(res2["count"])
		log.Printf("savedTime: %d newTime:%d, diff %d", firstTimeStamp, now, diff)
		if diff < int64(TIME_WINDOW) {
			counter++
			if counter < REQUESTS {
				data := map[string]interface{}{
					"timeStamp": res2["timeStamp"],
					"count":     strconv.Itoa(counter),
				}
				_, err := rl.client.HSet(ctx, ipAddr, data).Result()
				if err != nil {
					panic(err)
				}
			} else { // block it until the next window would come
				return true
			}

		} else {
			// reset counter and give a new time window
			data := map[string]interface{}{
				"timeStamp": time.Now().Unix(),
				"count":     "0",
			}
			_, err := rl.client.HSet(ctx, ipAddr, data).Result()
			if err != nil {
				panic(err)
			}
		}
	}
	return false
}

func (rl *RateLimiter) check(r *http.Request) bool {
	ip, err := rl.getIP(r)
	if err != nil {
		return false
	}
	// algo1
	return rl.algo1(ip)
}

func (rl *RateLimiter) getIP(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	netIP := net.ParseIP(splitIps[0])
	if netIP != nil {
		return netIP.String(), nil
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP = net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}

	return "", errors.New("IP not found")
}

func testapi(w http.ResponseWriter, r *http.Request) {
	if ok := rl.check(r); !ok {
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
