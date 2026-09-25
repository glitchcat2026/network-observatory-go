package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

var targets = []string{
	"google.com",
	"cloudflare.com",
	"github.com",
	"baidu.com",
}

type PingResult struct {
	MinRTT     int     `json:"MinRTT"`
	MaxRTT     int     `json:"MaxRTT"`
	AvgRTT     int     `json:"AvgRTT"`
	PacketLoss float64 `json:"PacketLoss"`
}

func main() {
	// 0. read config file
	for {
		now := time.Now()
		var results = make(map[string]PingResult, len(targets))
		for _, target := range targets {
			// 1. ping target
			pinger, err := probing.NewPinger(target)
			if err != nil {
				panic(err)
			}
			pinger.Count = 4
			pinger.Interval = time.Second
			err = pinger.Run()
			if err != nil {
				panic(err)
			}
			// 2. record min RTT, max RTT, avg RTT, packet loss rate into a map
			stats := pinger.Statistics()
			result := PingResult{
				MinRTT:     int(stats.MinRtt),
				MaxRTT:     int(stats.MaxRtt),
				AvgRTT:     int(stats.AvgRtt),
				PacketLoss: stats.PacketLoss,
			}
			results[target] = result
		}
		jsonData, err := json.Marshal(results)
		if err != nil {
			panic(err)
		}
		resp, err := http.Post("https://net-ob.glitchcat.top",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			panic(err)
		}
		fmt.Printf("[%s] %d\n", now, resp.StatusCode)
		time.Sleep(1 * time.Second)
		// 3. send the map to collector
		// 4. wait for 5 mins
	}
}
