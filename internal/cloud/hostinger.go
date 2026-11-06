package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var kvmPlan = regexp.MustCompile(`-vps-kvm\d+$`)

const hostingerBase = "https://developers.hostinger.com"

// Hostinger is the public Hostinger VPS API. Purchasing a virtual machine
// charges the Hostinger account. Listing and linking an existing machine does not.
type Hostinger struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func (h Hostinger) client() *http.Client {
	if h.HTTP != nil {
		return h.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (h Hostinger) base() string {
	if strings.TrimSpace(h.BaseURL) != "" {
		return strings.TrimRight(h.BaseURL, "/")
	}
	return hostingerBase
}

func (h Hostinger) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base()+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(h.Token))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("hostinger rate limit exceeded")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var env struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Message != "" {
			msg = env.Message
		}
		if msg == "" {
			msg = res.Status
		}
		return fmt.Errorf("hostinger: %s", msg)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// VirtualMachine is the subset Dockfin needs to register a server.
type VirtualMachine struct {
	ID    int    `json:"id"`
	State string `json:"state"`
	IPv4  []struct {
		Address string `json:"address"`
	} `json:"ipv4"`
	IPv6 []struct {
		Address string `json:"address"`
	} `json:"ipv6"`
}

func (v VirtualMachine) PublicIP() string {
	for _, ip := range v.IPv4 {
		if strings.TrimSpace(ip.Address) != "" {
			return ip.Address
		}
	}
	for _, ip := range v.IPv6 {
		if strings.TrimSpace(ip.Address) != "" {
			return ip.Address
		}
	}
	return ""
}

func (h Hostinger) VirtualMachines(ctx context.Context) ([]VirtualMachine, error) {
	var out []VirtualMachine
	if err := h.do(ctx, http.MethodGet, "/api/vps/v1/virtual-machines", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h Hostinger) DataCenters(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := h.do(ctx, http.MethodGet, "/api/vps/v1/data-centers", nil, &out)
	return out, err
}

func (h Hostinger) Templates(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := h.do(ctx, http.MethodGet, "/api/vps/v1/templates", nil, &out)
	return out, err
}

// Catalog returns KVM VPS plans. Game panel plans share the category and are excluded.
func (h Hostinger) Catalog(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := h.do(ctx, http.MethodGet, "/api/billing/v1/catalog?category=VPS", nil, &out); err != nil {
		return nil, err
	}
	var kvm []map[string]any
	for _, item := range out {
		id, _ := item["id"].(string)
		if kvmPlan.MatchString(id) {
			kvm = append(kvm, item)
		}
	}
	return kvm, nil
}

// PurchaseOrder is a paid Hostinger VPS order. ItemID, data center, and template are required.
type PurchaseOrder struct {
	ItemID       string
	DataCenterID int
	TemplateID   int
	Hostname     string
	PublicKey    string
	KeyName      string
}

func (h Hostinger) Purchase(ctx context.Context, order PurchaseOrder) (VirtualMachine, error) {
	if strings.TrimSpace(order.ItemID) == "" || order.DataCenterID == 0 || order.TemplateID == 0 {
		return VirtualMachine{}, fmt.Errorf("item_id, data_center_id, and template_id are required")
	}
	if strings.TrimSpace(order.PublicKey) == "" {
		return VirtualMachine{}, fmt.Errorf("public key is required")
	}
	setup := map[string]any{
		"data_center_id": order.DataCenterID,
		"template_id":    order.TemplateID,
		"public_key": map[string]string{
			"name": order.KeyName,
			"key":  order.PublicKey,
		},
	}
	if strings.Contains(order.Hostname, ".") {
		setup["hostname"] = order.Hostname
	}
	var response struct {
		ID             string         `json:"id"`
		Status         string         `json:"status"`
		Message        string         `json:"message"`
		VirtualMachine VirtualMachine `json:"virtual_machine"`
	}
	err := h.do(ctx, http.MethodPost, "/api/vps/v1/virtual-machines", map[string]any{
		"item_id": order.ItemID,
		"setup":   setup,
	}, &response)
	if err != nil {
		return VirtualMachine{}, err
	}
	if response.VirtualMachine.ID == 0 {
		msg := response.Message
		if msg == "" {
			msg = "Hostinger order " + response.ID + " is waiting for payment. Finish the VPS setup in hPanel."
		}
		return VirtualMachine{}, fmt.Errorf("%s", msg)
	}
	return response.VirtualMachine, nil
}
