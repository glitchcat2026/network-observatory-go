package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"network-observatory-go/internal"
	"os"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type ProbeConfig struct {
	CollectorURL       string   `json:"collector_url"`
	Hostnames          []string `json:"hostnames"`
	IPVersion          string   `json:"ip_version"`
	Token              string   `json:"token"`
	ProbeName          string   `json:"probe_name"`
	SleepSeconds       int      `json:"sleep_seconds"`
	PingTimeoutSeconds int      `json:"ping_timeout_seconds"`
}

const defaultPingTimeoutSeconds = 5

func loadConfig(path string) (ProbeConfig, bool, error) {
	var config ProbeConfig

	if _, err := os.Stat(path); os.IsNotExist(err) {
		config = ProbeConfig{
			CollectorURL:       "http://localhost:8080/api",
			Hostnames:          []string{"example.com"},
			IPVersion:          internal.IPVersionIPv4,
			Token:              internal.GenerateToken(),
			ProbeName:          "probe-1",
			SleepSeconds:       10,
			PingTimeoutSeconds: defaultPingTimeoutSeconds,
		}
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return config, false, fmt.Errorf("encode Probe config: %w", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return config, false, fmt.Errorf("write Probe config: %w", err)
		}
		return config, true, nil
	} else if err != nil {
		return config, false, fmt.Errorf("read Probe config: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return config, false, fmt.Errorf("read Probe config: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, false, fmt.Errorf("parse Probe config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values are not allowed")
		}
		return config, false, fmt.Errorf("parse Probe config: %w", err)
	}
	config.IPVersion, err = internal.NormalizeIPVersion(config.IPVersion)
	if err != nil {
		return config, false, err
	}
	if len(config.Hostnames) == 0 {
		return config, false, fmt.Errorf("hostnames must contain at least one target")
	}
	if config.PingTimeoutSeconds <= 0 {
		return config, false, fmt.Errorf("ping_timeout_seconds must be greater than zero")
	}
	if config.SleepSeconds <= 0 {
		return config, false, fmt.Errorf("sleep_seconds must be greater than zero")
	}

	return config, false, nil
}

func pingTarget(target string, ipVersion string, timeout time.Duration) (internal.PingResult, error) {
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return internal.PingResult{}, fmt.Errorf("create Ping instance: %w", err)
	}

	if ipVersion == internal.IPVersionIPv6 {
		pinger.SetNetwork("ip6")
	} else {
		pinger.SetNetwork("ip4")
	}
	pinger.SetPrivileged(true)
	pinger.Count = 4
	pinger.Interval = time.Second
	pinger.Timeout = timeout

	log.Printf("Starting Ping: %s", target)
	if err := pinger.Run(); err != nil {
		return internal.PingResult{}, fmt.Errorf("Ping failed: %w", err)
	}
	log.Printf("Ping finished: %s", target)

	stats := pinger.Statistics()

	result := internal.PingResult{
		Success:    stats.PacketsRecv > 0,
		MinRTT:     float64(stats.MinRtt) / float64(time.Millisecond),
		MaxRTT:     float64(stats.MaxRtt) / float64(time.Millisecond),
		AvgRTT:     float64(stats.AvgRtt) / float64(time.Millisecond),
		PacketLoss: stats.PacketLoss,
		Datetime:   time.Now().Format(time.RFC3339),
	}

	return result, nil
}

func sendReport(client *http.Client, config ProbeConfig, results map[string]internal.PingResult) error {
	report := internal.Report{
		Probe_name: config.ProbeName,
		Token:      config.Token,
		IPVersion:  config.IPVersion,
		Results:    results,
	}

	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("serialize report JSON: %w", err)
	}

	resp, err := client.Post(
		config.CollectorURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("send report: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[%s] Collector HTTP status: %d", time.Now().Format(time.RFC3339), resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Collector returned non-success status %d", resp.StatusCode)
	}

	return nil
}

func main() {
	// Load the configuration file, creating a template on first run.
	configPath, err := internal.ExecutableRelativePath("probe-config.json")
	if err != nil {
		log.Fatalf("resolve Probe config path: %v", err)
	}
	config, created, err := loadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		log.Print("probe-config.json created. Please edit it and restart.")
		return
	}
	// Set an HTTP timeout to avoid waiting indefinitely on network failures.
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	log.Printf("Probe started: name=%s, collector=%s", config.ProbeName, config.CollectorURL)

	// Probe and report continuously.
	for {
		results := make(map[string]internal.PingResult, len(config.Hostnames))

		for _, target := range config.Hostnames {
			result, err := pingTarget(target, config.IPVersion, time.Duration(config.PingTimeoutSeconds)*time.Second)
			if err != nil {
				log.Printf("Probe failed for %s: %v", target, err)
				continue
			}

			results[target] = result
			log.Printf("%s: avg=%.3f ms, loss=%.2f%%, success=%t",
				target, result.AvgRTT, result.PacketLoss, result.Success)
		}

		if len(results) > 0 {
			if err := sendReport(client, config, results); err != nil {
				log.Printf("Failed to report results: %v", err)
			}
		} else {
			log.Print("No probe results to report this round")
		}

		// Wait for the configured interval before the next round.
		time.Sleep(time.Duration(config.SleepSeconds) * time.Second)
	}
}
