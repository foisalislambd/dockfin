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
	Token string
	HTTP  *http.Client
	// Base overrides the API origin in tests. Empty uses the public Cloudflare API.
	Base string
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

func (c *CloudflareClient) base() string {
	if strings.TrimSpace(c.Base) != "" {
		return strings.TrimRight(c.Base, "/")
	}
	return "https://api.cloudflare.com/client/v4"
}

type CFZone struct {
	ID   string
	Name string
}

// ListZones returns active zones, following Cloudflare pagination.
func (c *CloudflareClient) ListZones() ([]CFZone, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("cloudflare token required")
	}
	var all []CFZone
	for page := 1; page <= 50; page++ {
		var out struct {
			Result []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"result"`
			ResultInfo struct {
				TotalPages int `json:"total_pages"`
			} `json:"result_info"`
		}
		path := fmt.Sprintf("/zones?status=active&per_page=50&page=%d", page)
		if err := c.do("GET", path, nil, &out); err != nil {
			return nil, err
		}
		for _, z := range out.Result {
			name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(z.Name)), ".")
			if z.ID == "" || name == "" {
				continue
			}
			all = append(all, CFZone{ID: z.ID, Name: name})
		}
		if out.ResultInfo.TotalPages == 0 || page >= out.ResultInfo.TotalPages {
			break
		}
	}
	return all, nil
}

// ListAddressRecords returns A, AAAA, and CNAME records for hostname.
func (c *CloudflareClient) ListAddressRecords(zoneID, hostname string) ([]RemoteRecord, error) {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records?per_page=100&name=" + url.QueryEscape(hostname)
	var out struct {
		Result []RemoteRecord `json:"result"`
	}
	if err := c.do("GET", path, nil, &out); err != nil {
		return nil, err
	}
	var recs []RemoteRecord
	for _, rec := range out.Result {
		switch strings.ToUpper(rec.Type) {
		case "A", "AAAA", "CNAME":
			rec.Name = strings.TrimSuffix(strings.ToLower(rec.Name), ".")
			rec.Type = strings.ToUpper(rec.Type)
			if rec.Name != hostname {
				continue
			}
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

// CreateRecord creates a DNS record and returns the provider record id.
func (c *CloudflareClient) CreateRecord(zoneID string, rec RemoteRecord) (string, error) {
	body := map[string]any{
		"type":    rec.Type,
		"name":    rec.Name,
		"content": rec.Content,
		"ttl":     1,
		"proxied": false,
		"comment": rec.Comment,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	var created struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := c.do("POST", "/zones/"+url.PathEscape(zoneID)+"/dns_records", raw, &created); err != nil {
		return "", err
	}
	if created.Result.ID == "" {
		return "", fmt.Errorf("cloudflare API: create returned no record id")
	}
	return created.Result.ID, nil
}

// UpdateContent changes only the record content. The ownership comment is left as stored.
func (c *CloudflareClient) UpdateContent(zoneID, recordID, content string) error {
	raw, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return err
	}
	return c.do("PATCH", "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), raw, nil)
}

// GetRecord loads one DNS record.
func (c *CloudflareClient) GetRecord(zoneID, recordID string) (RemoteRecord, error) {
	var out struct {
		Result RemoteRecord `json:"result"`
	}
	if err := c.do("GET", "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil, &out); err != nil {
		return RemoteRecord{}, err
	}
	out.Result.Type = strings.ToUpper(out.Result.Type)
	out.Result.Name = strings.TrimSuffix(strings.ToLower(out.Result.Name), ".")
	return out.Result, nil
}

// DeleteRecord removes a DNS record.
func (c *CloudflareClient) DeleteRecord(zoneID, recordID string) error {
	return c.do("DELETE", "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil, nil)
}

func (c *CloudflareClient) do(method, path string, body []byte, dest any) error {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.base()+path, rdr)
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
