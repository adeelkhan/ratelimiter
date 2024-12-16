package ratelimiter

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

const REQUESTS = 10
const TIME_WINDOW = 20

// stringify the rate limit config
// const API_RATELIMIT = "20/min"

// add block time
//const BLOCK_TIME = 60 //sec

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter() RateLimiter {
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

func (rl *RateLimiter) Check(r *http.Request) bool {
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
