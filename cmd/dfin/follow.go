package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func followDeployment(stdout, stderr io.Writer, call caller, base, token, team, id string, asJSON bool) int {
	code, out, err := call(base, token, team, http.MethodGet, "/api/v1/deployments/"+id, nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if code >= 400 {
		writeResult(asJSON, stdout, code, out)
		return 1
	}
	if !asJSON {
		fmt.Fprint(stdout, formatDeployment(out))
		if !strings.HasSuffix(formatDeployment(out), "\n") {
			fmt.Fprintln(stdout)
		}
	}
	var dep struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	_ = json.Unmarshal([]byte(out), &dep)
	if terminalDeployStatus(dep.Status) {
		if asJSON {
			fmt.Fprintln(stdout, strings.TrimSpace(out))
		}
		if dep.Status == "failed" {
			return 1
		}
		return 0
	}
	return followStream(stdout, stderr, base, token, team, id, asJSON)
}

func terminalDeployStatus(status string) bool {
	switch status {
	case "finished", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func followStream(stdout, stderr io.Writer, base, token, team, id string, asJSON bool) int {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/api/v1/deployments/"+id+"/logs/stream", nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if team != "" {
		req.Header.Set("X-Team-ID", team)
	}
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		writeResult(asJSON, stdout, resp.StatusCode, string(b))
		return 1
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var data strings.Builder
	exit := 0
	done := false
	flush := func() {
		payload := strings.TrimSpace(data.String())
		data.Reset()
		if payload == "" {
			return
		}
		if asJSON {
			fmt.Fprintln(stdout, payload)
		} else if line := sseLine(payload); line != "" {
			fmt.Fprintln(stdout, line)
		}
		var ev struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if json.Unmarshal([]byte(payload), &ev) == nil && ev.Type == "status" && terminalDeployStatus(ev.Status) {
			done = true
			if ev.Status == "failed" {
				exit = 1
				if !asJSON && ev.Error != "" {
					fmt.Fprintln(stdout, "error: "+ev.Error)
				}
			}
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			flush()
			if done {
				return exit
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return exit
}

func sseLine(payload string) string {
	var ev struct {
		Stage string `json:"stage"`
		Line  string `json:"line"`
		Type  string `json:"type"`
	}
	if json.Unmarshal([]byte(payload), &ev) != nil {
		return payload
	}
	if ev.Line == "" {
		return ""
	}
	if ev.Stage != "" && ev.Type != "status" {
		return "[" + ev.Stage + "] " + ev.Line
	}
	return ev.Line
}
