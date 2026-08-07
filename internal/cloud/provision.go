// Package cloud provisions VPS instances from the supported cloud providers
// (Hetzner Cloud, DigitalOcean, Vultr) using their public REST APIs.
package cloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProvisionRequest describes the instance to create. Region/Size are provider
// specific free-form identifiers (Hetzner location + server type, DigitalOcean
// region slug + size slug, Vultr region + plan).
type ProvisionRequest struct {
	Provider  string
	Token     string
	Name      string
	Region    string
	Size      string
	Image     string
	PublicKey string // OpenSSH authorized_keys line
	UserData  string // cloud-init; DefaultCloudInit() is used when empty
}

// ProvisionResult carries the provider identifiers plus the reachable IPv4.
type ProvisionResult struct {
	Provider   string `json:"provider"`
	InstanceID string `json:"instance_id"`
	SSHKeyID   string `json:"ssh_key_id"`
	IP         string `json:"ip"`
	Region     string `json:"region"`
	Size       string `json:"size"`
	Image      string `json:"image"`
}

// Defaults returns the region/size/image used when the caller leaves them blank.
func Defaults(provider string) (region, size, image string) {
	switch provider {
	case "hetzner":
		return "nbg1", "cpx11", "ubuntu-24.04"
	case "digitalocean":
		return "nyc3", "s-1vcpu-1gb", "ubuntu-24-04-x64"
	case "vultr":
		return "ewr", "vc2-1c-1gb", "2284"
	}
	return "", "", ""
}

// DefaultCloudInit installs Docker so the new host passes server validation
// right after provisioning.
const DefaultCloudInit = `#cloud-config
package_update: true
runcmd:
  - [ sh, -c, "curl -fsSL https://get.docker.com | sh" ]
  - [ systemctl, enable, --now, docker ]
`

// pollTimeout bounds how long we wait for the provider to hand out a public IP.
// The API route budget is 120s, so stay comfortably below it.
const pollTimeout = 90 * time.Second

const pollInterval = 4 * time.Second

// Provision ensures the SSH key exists at the provider, creates the instance,
// and waits until it has a public IPv4 address.
func Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Token = strings.TrimSpace(req.Token)
	req.Name = strings.TrimSpace(req.Name)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	if req.Token == "" {
		return nil, fmt.Errorf("cloud token is empty")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.PublicKey == "" {
		return nil, fmt.Errorf("private key has no public key material")
	}
	defRegion, defSize, defImage := Defaults(req.Provider)
	if req.Region == "" {
		req.Region = defRegion
	}
	if req.Size == "" {
		req.Size = defSize
	}
	if req.Image == "" {
		req.Image = defImage
	}
	if strings.TrimSpace(req.UserData) == "" {
		req.UserData = DefaultCloudInit
	}

	switch req.Provider {
	case "hetzner":
		return provisionHetzner(ctx, req)
	case "digitalocean":
		return provisionDigitalOcean(ctx, req)
	case "vultr":
		return provisionVultr(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider %q", req.Provider)
	}
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func doJSON(ctx context.Context, method, url, token string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d %s", method, redactURL(url), res.StatusCode, snippet(raw))
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func redactURL(u string) string {
	if i := strings.Index(u, "?"); i > 0 {
		return u[:i]
	}
	return u
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// sameSSHKey compares the algorithm + base64 body so trailing comments do not
// cause duplicate uploads.
func sameSSHKey(a, b string) bool {
	norm := func(s string) string {
		fields := strings.Fields(strings.TrimSpace(s))
		if len(fields) < 2 {
			return strings.TrimSpace(s)
		}
		return fields[0] + " " + fields[1]
	}
	return norm(a) == norm(b)
}

func keyName(base string) string {
	return fmt.Sprintf("dockfin-%s", strings.TrimSpace(base))
}

func waitForIP(ctx context.Context, fetch func(context.Context) (string, error)) (string, error) {
	deadline := time.Now().Add(pollTimeout)
	var lastErr error
	for {
		ip, err := fetch(ctx)
		if err != nil {
			lastErr = err
		} else if ip != "" && ip != "0.0.0.0" {
			return ip, nil
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return "", fmt.Errorf("timed out waiting for public IP: %w", lastErr)
			}
			return "", fmt.Errorf("timed out waiting for public IP after %s", pollTimeout)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// --- Hetzner Cloud ---

func provisionHetzner(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	const base = "https://api.hetzner.cloud/v1"
	var keys struct {
		SSHKeys []struct {
			ID        int64  `json:"id"`
			PublicKey string `json:"public_key"`
		} `json:"ssh_keys"`
	}
	if err := doJSON(ctx, http.MethodGet, base+"/ssh_keys?per_page=100", req.Token, nil, &keys); err != nil {
		return nil, err
	}
	var keyID int64
	for _, k := range keys.SSHKeys {
		if sameSSHKey(k.PublicKey, req.PublicKey) {
			keyID = k.ID
			break
		}
	}
	if keyID == 0 {
		var created struct {
			SSHKey struct {
				ID int64 `json:"id"`
			} `json:"ssh_key"`
		}
		body := map[string]string{"name": keyName(req.Name), "public_key": req.PublicKey}
		if err := doJSON(ctx, http.MethodPost, base+"/ssh_keys", req.Token, body, &created); err != nil {
			return nil, fmt.Errorf("upload ssh key: %w", err)
		}
		keyID = created.SSHKey.ID
	}

	var createdSrv struct {
		Server struct {
			ID        int64 `json:"id"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"server"`
	}
	body := map[string]any{
		"name":               req.Name,
		"server_type":        req.Size,
		"image":              req.Image,
		"location":           req.Region,
		"ssh_keys":           []int64{keyID},
		"user_data":          req.UserData,
		"start_after_create": true,
	}
	if err := doJSON(ctx, http.MethodPost, base+"/servers", req.Token, body, &createdSrv); err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	instanceID := strconv.FormatInt(createdSrv.Server.ID, 10)
	ip := createdSrv.Server.PublicNet.IPv4.IP
	if ip == "" {
		var err error
		ip, err = waitForIP(ctx, func(ctx context.Context) (string, error) {
			var got struct {
				Server struct {
					PublicNet struct {
						IPv4 struct {
							IP string `json:"ip"`
						} `json:"ipv4"`
					} `json:"public_net"`
				} `json:"server"`
			}
			if err := doJSON(ctx, http.MethodGet, base+"/servers/"+instanceID, req.Token, nil, &got); err != nil {
				return "", err
			}
			return got.Server.PublicNet.IPv4.IP, nil
		})
		if err != nil {
			return nil, fmt.Errorf("hetzner server %s: %w", instanceID, err)
		}
	}
	return &ProvisionResult{
		Provider:   "hetzner",
		InstanceID: instanceID,
		SSHKeyID:   strconv.FormatInt(keyID, 10),
		IP:         ip,
		Region:     req.Region,
		Size:       req.Size,
		Image:      req.Image,
	}, nil
}

// --- DigitalOcean ---

func provisionDigitalOcean(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	const base = "https://api.digitalocean.com/v2"
	var keys struct {
		SSHKeys []struct {
			ID        int64  `json:"id"`
			PublicKey string `json:"public_key"`
		} `json:"ssh_keys"`
	}
	if err := doJSON(ctx, http.MethodGet, base+"/account/keys?per_page=200", req.Token, nil, &keys); err != nil {
		return nil, err
	}
	var keyID int64
	for _, k := range keys.SSHKeys {
		if sameSSHKey(k.PublicKey, req.PublicKey) {
			keyID = k.ID
			break
		}
	}
	if keyID == 0 {
		var created struct {
			SSHKey struct {
				ID int64 `json:"id"`
			} `json:"ssh_key"`
		}
		body := map[string]string{"name": keyName(req.Name), "public_key": req.PublicKey}
		if err := doJSON(ctx, http.MethodPost, base+"/account/keys", req.Token, body, &created); err != nil {
			return nil, fmt.Errorf("upload ssh key: %w", err)
		}
		keyID = created.SSHKey.ID
	}

	var created struct {
		Droplet struct {
			ID int64 `json:"id"`
		} `json:"droplet"`
	}
	body := map[string]any{
		"name":      req.Name,
		"region":    req.Region,
		"size":      req.Size,
		"image":     req.Image,
		"ssh_keys":  []int64{keyID},
		"user_data": req.UserData,
	}
	if err := doJSON(ctx, http.MethodPost, base+"/droplets", req.Token, body, &created); err != nil {
		return nil, fmt.Errorf("create droplet: %w", err)
	}
	instanceID := strconv.FormatInt(created.Droplet.ID, 10)
	ip, err := waitForIP(ctx, func(ctx context.Context) (string, error) {
		var got struct {
			Droplet struct {
				Networks struct {
					V4 []struct {
						IPAddress string `json:"ip_address"`
						Type      string `json:"type"`
					} `json:"v4"`
				} `json:"networks"`
			} `json:"droplet"`
		}
		if err := doJSON(ctx, http.MethodGet, base+"/droplets/"+instanceID, req.Token, nil, &got); err != nil {
			return "", err
		}
		for _, n := range got.Droplet.Networks.V4 {
			if n.Type == "public" {
				return n.IPAddress, nil
			}
		}
		return "", nil
	})
	if err != nil {
		return nil, fmt.Errorf("digitalocean droplet %s: %w", instanceID, err)
	}
	return &ProvisionResult{
		Provider:   "digitalocean",
		InstanceID: instanceID,
		SSHKeyID:   strconv.FormatInt(keyID, 10),
		IP:         ip,
		Region:     req.Region,
		Size:       req.Size,
		Image:      req.Image,
	}, nil
}

// --- Vultr ---

func provisionVultr(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	const base = "https://api.vultr.com/v2"
	var keys struct {
		SSHKeys []struct {
			ID     string `json:"id"`
			SSHKey string `json:"ssh_key"`
		} `json:"ssh_keys"`
	}
	if err := doJSON(ctx, http.MethodGet, base+"/ssh-keys?per_page=500", req.Token, nil, &keys); err != nil {
		return nil, err
	}
	var keyID string
	for _, k := range keys.SSHKeys {
		if sameSSHKey(k.SSHKey, req.PublicKey) {
			keyID = k.ID
			break
		}
	}
	if keyID == "" {
		var created struct {
			SSHKey struct {
				ID string `json:"id"`
			} `json:"ssh_key"`
		}
		body := map[string]string{"name": keyName(req.Name), "ssh_key": req.PublicKey}
		if err := doJSON(ctx, http.MethodPost, base+"/ssh-keys", req.Token, body, &created); err != nil {
			return nil, fmt.Errorf("upload ssh key: %w", err)
		}
		keyID = created.SSHKey.ID
	}

	body := map[string]any{
		"region":    req.Region,
		"plan":      req.Size,
		"label":     req.Name,
		"hostname":  req.Name,
		"sshkey_id": []string{keyID},
		"user_data": base64.StdEncoding.EncodeToString([]byte(req.UserData)),
		"backups":   "disabled",
	}
	// Numeric images are Vultr OS ids; anything else is treated as a marketplace image id.
	if osID, err := strconv.Atoi(strings.TrimSpace(req.Image)); err == nil {
		body["os_id"] = osID
	} else {
		body["image_id"] = req.Image
	}
	var created struct {
		Instance struct {
			ID     string `json:"id"`
			MainIP string `json:"main_ip"`
		} `json:"instance"`
	}
	if err := doJSON(ctx, http.MethodPost, base+"/instances", req.Token, body, &created); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	instanceID := created.Instance.ID
	ip, err := waitForIP(ctx, func(ctx context.Context) (string, error) {
		var got struct {
			Instance struct {
				MainIP string `json:"main_ip"`
			} `json:"instance"`
		}
		if err := doJSON(ctx, http.MethodGet, base+"/instances/"+instanceID, req.Token, nil, &got); err != nil {
			return "", err
		}
		return got.Instance.MainIP, nil
	})
	if err != nil {
		return nil, fmt.Errorf("vultr instance %s: %w", instanceID, err)
	}
	return &ProvisionResult{
		Provider:   "vultr",
		InstanceID: instanceID,
		SSHKeyID:   keyID,
		IP:         ip,
		Region:     req.Region,
		Size:       req.Size,
		Image:      req.Image,
	}, nil
}
