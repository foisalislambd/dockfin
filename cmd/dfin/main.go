package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	base := env("DOCKFIN_URL", "http://localhost:8000")
	token := env("DOCKFIN_TOKEN", "")
	switch os.Args[1] {
	case "version":
		fmt.Println("dfin", version.Version)
	case "health":
		url := base + "/health"
		if len(os.Args) > 2 {
			url = os.Args[2]
		}
		resp, err := http.Get(url)
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		fmt.Println(resp.StatusCode, string(b))
	case "deploy":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: dfin deploy <application-uuid>"))
		}
		appID := os.Args[2]
		body := map[string]any{"force_rebuild": false}
		code, out := api(base, token, http.MethodPost, "/api/v1/applications/"+appID+"/deploy", body)
		fmt.Println(code, out)
	case "stop":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: dfin stop <application-uuid>"))
		}
		code, out := api(base, token, http.MethodPost, "/api/v1/applications/"+os.Args[2]+"/stop", nil)
		fmt.Println(code, out)
	case "logs":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: dfin logs <deployment-uuid>"))
		}
		code, out := api(base, token, http.MethodGet, "/api/v1/deployments/"+os.Args[2], nil)
		fmt.Println(code, out)
	case "apps":
		code, out := api(base, token, http.MethodGet, "/api/v1/applications", nil)
		fmt.Println(code, out)
	case "services":
		code, out := api(base, token, http.MethodGet, "/api/v1/services", nil)
		fmt.Println(code, out)
	case "databases":
		code, out := api(base, token, http.MethodGet, "/api/v1/databases", nil)
		fmt.Println(code, out)
	case "servers":
		code, out := api(base, token, http.MethodGet, "/api/v1/servers", nil)
		fmt.Println(code, out)
	case "service-deploy":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: dfin service-deploy <service-uuid>"))
		}
		code, out := api(base, token, http.MethodPost, "/api/v1/services/"+os.Args[2]+"/deploy", nil)
		fmt.Println(code, out)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`dfin — Dockfin CLI

Usage:
  dfin version
  dfin health [url]
  dfin servers
  dfin apps
  dfin services
  dfin databases
  dfin deploy <application-uuid>
  dfin stop <application-uuid>
  dfin service-deploy <service-uuid>
  dfin logs <deployment-uuid>

Env:
  DOCKFIN_URL    default http://localhost:8000
  DOCKFIN_TOKEN  session/API bearer token
`)
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func api(base, token, method, path string, body any) (int, string) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, rdr)
	if err != nil {
		fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}
