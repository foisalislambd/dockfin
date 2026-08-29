package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CloudflareClient struct {
	Token  string
	HTTP   *http.Client
}

func NewCloudflare(token string) *CloudflareClient {
	return &CloudflareClient{
		Token: strings.TrimSpace(token),
		HTTP:  &http.Client{Timeout: 20 * time.Second},
	}
}

type UpsertResult struct {
	Zone     string `json:"zone"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	Action   string `json:"action"`
	RecordID string `json:"record_id"`
}

// UpsertA creates or updates an A record for hostname pointing at ipv4.
func (c *CloudflareClient) UpsertA(hostname, ipv4 string) (*UpsertResult, error) {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	ipv4 = strings.TrimSpace(ipv4)
	if hostname == "" || ipv4 == "" {
		return nil, fmt.Errorf("hostname and ipv4 required")
	}
	if c.Token == "" {
		return nil, fmt.Errorf("cloudflare token required")
	}
	zoneName, zoneID, err := c.resolveZone(hostname)
	if err != nil {
		return nil, err
	}
	existingID, err := c.findA(zoneID, hostname)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"type":    "A",
		"name":    hostname,
		"content": ipv4,
		"ttl":     1, // auto
		"proxied": false,
	}
	raw, _ := json.Marshal(body)
	if existingID != "" {
		if err := c.do("PUT", "/zones/"+zoneID+"/dns_records/"+existingID, raw, nil); err != nil {
			return nil, err
		}
		return &UpsertResult{Zone: zoneName, Name: hostname, Type: "A", Content: ipv4, Action: "updated", RecordID: existingID}, nil
	}
	var created struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := c.do("POST", "/zones/"+zoneID+"/dns_records", raw, &created); err != nil {
		return nil, err
	}
	return &UpsertResult{Zone: zoneName, Name: hostname, Type: "A", Content: ipv4, Action: "created", RecordID: created.Result.ID}, nil
}

func zoneFromHost(host string) string {
	cands := candidateZones(host)
	if len(cands) == 0 {
		return host
	}
	return cands[0]
}

func candidateZones(host string) []string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		if host == "" {
			return nil
		}
		return []string{host}
	}
	out := make([]string, 0, len(parts)-1)
	for i := 0; i <= len(parts)-2; i++ {
		out = append(out, strings.Join(parts[i:], "."))
	}
	return out
}

func (c *CloudflareClient) resolveZone(hostname string) (string, string, error) {
	var last error
	for _, name := range candidateZones(hostname) {
		id, err := c.zoneID(name)
		if err == nil {
			return name, id, nil
		}
		last = err
	}
	if last != nil {
		return "", "", last
	}
	return "", "", fmt.Errorf("cloudflare zone not found for %q", hostname)
}

func (c *CloudflareClient) zoneID(zoneName string) (string, error) {
	var out struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	if err := c.do("GET", "/zones?name="+url.QueryEscape(zoneName)+"&status=active", nil, &out); err != nil {
		return "", err
	}
	for _, z := range out.Result {
		if strings.EqualFold(z.Name, zoneName) {
			return z.ID, nil
		}
	}
	if len(out.Result) == 1 {
		return out.Result[0].ID, nil
	}
	return "", fmt.Errorf("cloudflare zone %q not found (token needs Zone:Read + DNS:Edit)", zoneName)
}

func (c *CloudflareClient) findA(zoneID, hostname string) (string, error) {
	var out struct {
		Result []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := c.do("GET", "/zones/"+zoneID+"/dns_records?type=A&name="+url.QueryEscape(hostname), nil, &out); err != nil {
		return "", err
	}
	if len(out.Result) == 0 {
		return "", nil
	}
	return out.Result[0].ID, nil
}

func (c *CloudflareClient) do(method, path string, body []byte, dest any) error {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "https://api.cloudflare.com/client/v4"+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("cloudflare API HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var check struct {
		Success *bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &check); err == nil && check.Success != nil && !*check.Success {
		msg := "request failed"
		if len(check.Errors) > 0 && check.Errors[0].Message != "" {
			msg = check.Errors[0].Message
		}
		return fmt.Errorf("cloudflare API: %s", msg)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
