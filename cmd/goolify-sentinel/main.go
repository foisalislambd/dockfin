package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"
)

// goolify-sentinel pushes host metrics to the control plane.
func main() {
	url := env("GOOLIFY_URL", "")
	token := env("GOOLIFY_TOKEN", "")
	serverID := env("GOOLIFY_SERVER_ID", "")
	if url == "" || serverID == "" {
		fmt.Fprintln(os.Stderr, "GOOLIFY_URL and GOOLIFY_SERVER_ID required")
		os.Exit(1)
	}
	interval := 30 * time.Second
	for {
		payload := map[string]any{
			"server_id":          serverID,
			"token":              token,
			"cpu_percent":        float64(runtime.NumCPU()),
			"memory_used_bytes":  0,
			"memory_total_bytes": 0,
			"disk_used_bytes":    0,
			"disk_total_bytes":   0,
		}
		b, _ := json.Marshal(payload)
		req, err := http.NewRequest(http.MethodPost, stringsTrim(url)+"/api/v1/sentinel/metrics", bytes.NewReader(b))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
		time.Sleep(interval)
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func stringsTrim(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
