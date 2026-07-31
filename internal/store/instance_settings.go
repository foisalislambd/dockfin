package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InstanceSettings is the singleton instance-wide configuration (Coolify parity).
type InstanceSettings struct {
	ID                         int16     `json:"id"`
	PublicURL                  string    `json:"public_url"`
	InstanceName               string    `json:"instance_name"`
	InstanceTimezone           string    `json:"instance_timezone"`
	PublicIPv4                 string    `json:"public_ipv4"`
	PublicIPv6                 string    `json:"public_ipv6"`
	IsRegistrationEnabled      bool      `json:"is_registration_enabled"`
	DoNotTrack                 bool      `json:"do_not_track"`
	IsDNSValidationEnabled     bool      `json:"is_dns_validation_enabled"`
	CustomDNSServers           string    `json:"custom_dns_servers"`
	IsAPIEnabled               bool      `json:"is_api_enabled"`
	AllowedIPs                 string    `json:"allowed_ips"`
	WebhookAllowedInternalHosts string   `json:"webhook_allowed_internal_hosts"`
	WebhookAllowLocalhost      bool      `json:"webhook_allow_localhost"`
	IsMCPServerEnabled         bool      `json:"is_mcp_server_enabled"`
	DisableTwoStepConfirmation bool      `json:"disable_two_step_confirmation"`
	IsSponsorshipPopupEnabled  bool      `json:"is_sponsorship_popup_enabled"`
	UpdateChannel              string    `json:"update_channel"`
	IsAutoUpdateEnabled        bool      `json:"is_auto_update_enabled"`
	AutoUpdateFrequency        string    `json:"auto_update_frequency"`
	UpdateCheckFrequency       string    `json:"update_check_frequency"`
	DockerRegistryURL          string    `json:"docker_registry_url"`
	SMTPEnabled                bool      `json:"smtp_enabled"`
	SMTPFromName               string    `json:"smtp_from_name"`
	SMTPFromAddress            string    `json:"smtp_from_address"`
	SMTPHost                   string    `json:"smtp_host"`
	SMTPPort                   int       `json:"smtp_port"`
	SMTPEncryption             string    `json:"smtp_encryption"`
	SMTPUsername               string    `json:"smtp_username,omitempty"`
	SMTPPasswordSet            bool      `json:"smtp_password_set"`
	SMTPTimeout                *int      `json:"smtp_timeout"`
	ResendEnabled              bool      `json:"resend_enabled"`
	ResendAPIKeySet            bool      `json:"resend_api_key_set"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// InstanceSettingsPatch is a partial update for instance settings.
type InstanceSettingsPatch struct {
	PublicURL                   *string `json:"public_url"`
	InstanceName                *string `json:"instance_name"`
	InstanceTimezone            *string `json:"instance_timezone"`
	PublicIPv4                  *string `json:"public_ipv4"`
	PublicIPv6                  *string `json:"public_ipv6"`
	IsRegistrationEnabled       *bool   `json:"is_registration_enabled"`
	DoNotTrack                  *bool   `json:"do_not_track"`
	IsDNSValidationEnabled      *bool   `json:"is_dns_validation_enabled"`
	CustomDNSServers            *string `json:"custom_dns_servers"`
	IsAPIEnabled                *bool   `json:"is_api_enabled"`
	AllowedIPs                  *string `json:"allowed_ips"`
	WebhookAllowedInternalHosts *string `json:"webhook_allowed_internal_hosts"`
	WebhookAllowLocalhost       *bool   `json:"webhook_allow_localhost"`
	IsMCPServerEnabled          *bool   `json:"is_mcp_server_enabled"`
	DisableTwoStepConfirmation  *bool   `json:"disable_two_step_confirmation"`
	IsSponsorshipPopupEnabled   *bool   `json:"is_sponsorship_popup_enabled"`
	UpdateChannel               *string `json:"update_channel"`
	IsAutoUpdateEnabled         *bool   `json:"is_auto_update_enabled"`
	AutoUpdateFrequency         *string `json:"auto_update_frequency"`
	UpdateCheckFrequency        *string `json:"update_check_frequency"`
	DockerRegistryURL           *string `json:"docker_registry_url"`
	SMTPEnabled                 *bool   `json:"smtp_enabled"`
	SMTPFromName                *string `json:"smtp_from_name"`
	SMTPFromAddress             *string `json:"smtp_from_address"`
	SMTPHost                    *string `json:"smtp_host"`
	SMTPPort                    *int    `json:"smtp_port"`
	SMTPEncryption              *string `json:"smtp_encryption"`
	SMTPUsername                *string `json:"smtp_username"`
	SMTPPassword                *string `json:"smtp_password"`
	SMTPTimeout                 *int    `json:"smtp_timeout"`
	ResendEnabled               *bool   `json:"resend_enabled"`
	ResendAPIKey                *string `json:"resend_api_key"`
}

type OauthSetting struct {
	ID              uuid.UUID `json:"id"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	ClientID        string    `json:"client_id"`
	ClientSecretSet bool      `json:"client_secret_set"`
	RedirectURI     string    `json:"redirect_uri"`
	Tenant          string    `json:"tenant"`
	BaseURL         string    `json:"base_url"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type OauthSettingPatch struct {
	Enabled      *bool   `json:"enabled"`
	ClientID     *string `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	RedirectURI  *string `json:"redirect_uri"`
	Tenant       *string `json:"tenant"`
	BaseURL      *string `json:"base_url"`
}

const instanceSettingsCols = `
	id, public_url, instance_name, instance_timezone, public_ipv4, public_ipv6,
	is_registration_enabled, do_not_track, is_dns_validation_enabled, custom_dns_servers,
	is_api_enabled, allowed_ips, webhook_allowed_internal_hosts, webhook_allow_localhost,
	is_mcp_server_enabled, disable_two_step_confirmation, is_sponsorship_popup_enabled,
	update_channel, is_auto_update_enabled, auto_update_frequency, update_check_frequency,
	docker_registry_url, smtp_enabled, smtp_from_name, smtp_from_address, smtp_host, smtp_port,
	smtp_encryption, smtp_username_enc, smtp_password_enc, smtp_timeout, resend_enabled,
	resend_api_key_enc, updated_at`

func (s *Store) GetInstanceSettings(ctx context.Context) (*InstanceSettings, error) {
	var (
		st             InstanceSettings
		smtpUserEnc    string
		smtpPassEnc    string
		resendKeyEnc   string
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT `+instanceSettingsCols+`
		FROM instance_settings WHERE id = 1
	`).Scan(
		&st.ID, &st.PublicURL, &st.InstanceName, &st.InstanceTimezone, &st.PublicIPv4, &st.PublicIPv6,
		&st.IsRegistrationEnabled, &st.DoNotTrack, &st.IsDNSValidationEnabled, &st.CustomDNSServers,
		&st.IsAPIEnabled, &st.AllowedIPs, &st.WebhookAllowedInternalHosts, &st.WebhookAllowLocalhost,
		&st.IsMCPServerEnabled, &st.DisableTwoStepConfirmation, &st.IsSponsorshipPopupEnabled,
		&st.UpdateChannel, &st.IsAutoUpdateEnabled, &st.AutoUpdateFrequency, &st.UpdateCheckFrequency,
		&st.DockerRegistryURL, &st.SMTPEnabled, &st.SMTPFromName, &st.SMTPFromAddress, &st.SMTPHost, &st.SMTPPort,
		&st.SMTPEncryption, &smtpUserEnc, &smtpPassEnc, &st.SMTPTimeout, &st.ResendEnabled,
		&resendKeyEnc, &st.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.SMTPPasswordSet = smtpPassEnc != ""
	st.ResendAPIKeySet = resendKeyEnc != ""
	if smtpUserEnc != "" && s.Box != nil {
		if plain, err := s.Box.DecryptString(smtpUserEnc); err == nil {
			st.SMTPUsername = plain
		}
	}
	return &st, nil
}

func (s *Store) UpdateInstanceSettings(ctx context.Context, patch InstanceSettingsPatch) (*InstanceSettings, error) {
	cur, err := s.GetInstanceSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := applyInstanceSettingsPatch(cur, &patch); err != nil {
		return nil, err
	}

	var smtpUserEnc, smtpPassEnc, resendKeyEnc string
	err = s.Pool.QueryRow(ctx, `
		SELECT smtp_username_enc, smtp_password_enc, resend_api_key_enc
		FROM instance_settings WHERE id = 1
	`).Scan(&smtpUserEnc, &smtpPassEnc, &resendKeyEnc)
	if err != nil {
		return nil, err
	}

	if patch.SMTPUsername != nil {
		v := strings.TrimSpace(*patch.SMTPUsername)
		if v == "" {
			smtpUserEnc = ""
		} else {
			if s.Box == nil {
				return nil, fmt.Errorf("encryption box not configured")
			}
			enc, err := s.Box.EncryptString(v)
			if err != nil {
				return nil, err
			}
			smtpUserEnc = enc
		}
	}
	if patch.SMTPPassword != nil {
		v := *patch.SMTPPassword
		if v == "" {
			smtpPassEnc = ""
		} else {
			if s.Box == nil {
				return nil, fmt.Errorf("encryption box not configured")
			}
			enc, err := s.Box.EncryptString(v)
			if err != nil {
				return nil, err
			}
			smtpPassEnc = enc
		}
	}
	if patch.ResendAPIKey != nil {
		v := strings.TrimSpace(*patch.ResendAPIKey)
		if v == "" {
			resendKeyEnc = ""
		} else {
			if s.Box == nil {
				return nil, fmt.Errorf("encryption box not configured")
			}
			enc, err := s.Box.EncryptString(v)
			if err != nil {
				return nil, err
			}
			resendKeyEnc = enc
		}
	}

	_, err = s.Pool.Exec(ctx, `
		UPDATE instance_settings SET
			public_url=$1, instance_name=$2, instance_timezone=$3, public_ipv4=$4, public_ipv6=$5,
			is_registration_enabled=$6, do_not_track=$7, is_dns_validation_enabled=$8, custom_dns_servers=$9,
			is_api_enabled=$10, allowed_ips=$11, webhook_allowed_internal_hosts=$12, webhook_allow_localhost=$13,
			is_mcp_server_enabled=$14, disable_two_step_confirmation=$15, is_sponsorship_popup_enabled=$16,
			update_channel=$17, is_auto_update_enabled=$18, auto_update_frequency=$19, update_check_frequency=$20,
			docker_registry_url=$21, smtp_enabled=$22, smtp_from_name=$23, smtp_from_address=$24, smtp_host=$25,
			smtp_port=$26, smtp_encryption=$27, smtp_username_enc=$28, smtp_password_enc=$29, smtp_timeout=$30,
			resend_enabled=$31, resend_api_key_enc=$32, updated_at=NOW()
		WHERE id = 1
	`,
		cur.PublicURL, cur.InstanceName, cur.InstanceTimezone, cur.PublicIPv4, cur.PublicIPv6,
		cur.IsRegistrationEnabled, cur.DoNotTrack, cur.IsDNSValidationEnabled, cur.CustomDNSServers,
		cur.IsAPIEnabled, cur.AllowedIPs, cur.WebhookAllowedInternalHosts, cur.WebhookAllowLocalhost,
		cur.IsMCPServerEnabled, cur.DisableTwoStepConfirmation, cur.IsSponsorshipPopupEnabled,
		cur.UpdateChannel, cur.IsAutoUpdateEnabled, cur.AutoUpdateFrequency, cur.UpdateCheckFrequency,
		cur.DockerRegistryURL, cur.SMTPEnabled, cur.SMTPFromName, cur.SMTPFromAddress, cur.SMTPHost,
		cur.SMTPPort, cur.SMTPEncryption, smtpUserEnc, smtpPassEnc, cur.SMTPTimeout,
		cur.ResendEnabled, resendKeyEnc,
	)
	if err != nil {
		return nil, err
	}
	return s.GetInstanceSettings(ctx)
}

func applyInstanceSettingsPatch(cur *InstanceSettings, patch *InstanceSettingsPatch) error {
	if patch.PublicURL != nil {
		v := strings.TrimSpace(*patch.PublicURL)
		if v != "" {
			u, err := url.Parse(v)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("%w: public_url must be a valid URL (e.g. https://dash.example.com)", ErrConflict)
			}
			cur.PublicURL = u.Scheme + "://" + u.Host
		} else {
			cur.PublicURL = ""
		}
	}
	if patch.InstanceName != nil {
		cur.InstanceName = strings.TrimSpace(*patch.InstanceName)
		if cur.InstanceName == "" {
			cur.InstanceName = "Goolify"
		}
	}
	if patch.InstanceTimezone != nil {
		tz := strings.TrimSpace(*patch.InstanceTimezone)
		if tz == "" {
			tz = "UTC"
		}
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("%w: invalid timezone", ErrConflict)
		}
		cur.InstanceTimezone = tz
	}
	if patch.PublicIPv4 != nil {
		v := strings.TrimSpace(*patch.PublicIPv4)
		if v != "" && net.ParseIP(v) == nil {
			return fmt.Errorf("%w: invalid public_ipv4", ErrConflict)
		}
		if v != "" && net.ParseIP(v).To4() == nil {
			return fmt.Errorf("%w: public_ipv4 must be IPv4", ErrConflict)
		}
		cur.PublicIPv4 = v
	}
	if patch.PublicIPv6 != nil {
		v := strings.TrimSpace(*patch.PublicIPv6)
		if v != "" {
			ip := net.ParseIP(v)
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("%w: invalid public_ipv6", ErrConflict)
			}
		}
		cur.PublicIPv6 = v
	}
	setBool := func(dst *bool, src *bool) {
		if src != nil {
			*dst = *src
		}
	}
	setString := func(dst *string, src *string) {
		if src != nil {
			*dst = strings.TrimSpace(*src)
		}
	}
	setBool(&cur.IsRegistrationEnabled, patch.IsRegistrationEnabled)
	setBool(&cur.DoNotTrack, patch.DoNotTrack)
	setBool(&cur.IsDNSValidationEnabled, patch.IsDNSValidationEnabled)
	setString(&cur.CustomDNSServers, patch.CustomDNSServers)
	if patch.CustomDNSServers != nil && cur.CustomDNSServers != "" {
		for _, part := range strings.Split(cur.CustomDNSServers, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if net.ParseIP(part) == nil {
				return fmt.Errorf("%w: invalid DNS server %q", ErrConflict, part)
			}
		}
	}
	setBool(&cur.IsAPIEnabled, patch.IsAPIEnabled)
	setString(&cur.AllowedIPs, patch.AllowedIPs)
	setString(&cur.WebhookAllowedInternalHosts, patch.WebhookAllowedInternalHosts)
	setBool(&cur.WebhookAllowLocalhost, patch.WebhookAllowLocalhost)
	setBool(&cur.IsMCPServerEnabled, patch.IsMCPServerEnabled)
	setBool(&cur.DisableTwoStepConfirmation, patch.DisableTwoStepConfirmation)
	setBool(&cur.IsSponsorshipPopupEnabled, patch.IsSponsorshipPopupEnabled)
	if patch.UpdateChannel != nil {
		ch := strings.TrimSpace(*patch.UpdateChannel)
		if ch != "stable" && ch != "next" && ch != "nightly" {
			return fmt.Errorf("%w: update_channel must be stable, next, or nightly", ErrConflict)
		}
		cur.UpdateChannel = ch
	}
	setBool(&cur.IsAutoUpdateEnabled, patch.IsAutoUpdateEnabled)
	setString(&cur.AutoUpdateFrequency, patch.AutoUpdateFrequency)
	setString(&cur.UpdateCheckFrequency, patch.UpdateCheckFrequency)
	if patch.DockerRegistryURL != nil {
		reg := strings.TrimSpace(*patch.DockerRegistryURL)
		if reg != "docker.io" && reg != "ghcr.io" {
			return fmt.Errorf("%w: docker_registry_url must be docker.io or ghcr.io", ErrConflict)
		}
		cur.DockerRegistryURL = reg
	}
	setBool(&cur.SMTPEnabled, patch.SMTPEnabled)
	if patch.SMTPFromName != nil {
		setString(&cur.SMTPFromName, patch.SMTPFromName)
	}
	if patch.SMTPFromAddress != nil {
		setString(&cur.SMTPFromAddress, patch.SMTPFromAddress)
		if cur.SMTPFromAddress != "" {
			if _, err := mail.ParseAddress(cur.SMTPFromAddress); err != nil {
				return fmt.Errorf("%w: invalid smtp_from_address", ErrConflict)
			}
		}
	}
	if patch.SMTPHost != nil {
		setString(&cur.SMTPHost, patch.SMTPHost)
	}
	if patch.SMTPPort != nil {
		if *patch.SMTPPort < 1 || *patch.SMTPPort > 65535 {
			return fmt.Errorf("%w: smtp_port out of range", ErrConflict)
		}
		cur.SMTPPort = *patch.SMTPPort
	}
	if patch.SMTPEncryption != nil {
		enc := strings.TrimSpace(*patch.SMTPEncryption)
		if enc != "starttls" && enc != "tls" && enc != "none" {
			return fmt.Errorf("%w: smtp_encryption must be starttls, tls, or none", ErrConflict)
		}
		cur.SMTPEncryption = enc
	}
	if patch.SMTPTimeout != nil {
		cur.SMTPTimeout = patch.SMTPTimeout
	}
	setBool(&cur.ResendEnabled, patch.ResendEnabled)
	// SMTP and Resend are mutually exclusive.
	if cur.SMTPEnabled && cur.ResendEnabled {
		if patch.ResendEnabled != nil && *patch.ResendEnabled {
			cur.SMTPEnabled = false
		} else {
			cur.ResendEnabled = false
		}
	}
	return nil
}

func (s *Store) RegistrationEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.Pool.QueryRow(ctx, `SELECT is_registration_enabled FROM instance_settings WHERE id = 1`).Scan(&enabled)
	return enabled, err
}

func (s *Store) ListOauthSettings(ctx context.Context) ([]OauthSetting, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, provider, enabled, client_id, client_secret_enc, redirect_uri, tenant, base_url, updated_at
		FROM oauth_settings ORDER BY provider
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OauthSetting
	for rows.Next() {
		var o OauthSetting
		var secretEnc string
		if err := rows.Scan(&o.ID, &o.Provider, &o.Enabled, &o.ClientID, &secretEnc, &o.RedirectURI, &o.Tenant, &o.BaseURL, &o.UpdatedAt); err != nil {
			return nil, err
		}
		o.ClientSecretSet = secretEnc != ""
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) UpdateOauthSetting(ctx context.Context, provider string, patch OauthSettingPatch) (*OauthSetting, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var (
		id         uuid.UUID
		enabled    bool
		clientID   string
		secretEnc  string
		redirect   string
		tenant     string
		baseURL    string
		updatedAt  time.Time
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT id, enabled, client_id, client_secret_enc, redirect_uri, tenant, base_url, updated_at
		FROM oauth_settings WHERE provider = $1
	`, provider).Scan(&id, &enabled, &clientID, &secretEnc, &redirect, &tenant, &baseURL, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if patch.Enabled != nil {
		enabled = *patch.Enabled
	}
	if patch.ClientID != nil {
		clientID = strings.TrimSpace(*patch.ClientID)
	}
	if patch.RedirectURI != nil {
		redirect = strings.TrimSpace(*patch.RedirectURI)
	}
	if patch.Tenant != nil {
		tenant = strings.TrimSpace(*patch.Tenant)
	}
	if patch.BaseURL != nil {
		baseURL = strings.TrimSpace(*patch.BaseURL)
	}
	if patch.ClientSecret != nil {
		v := *patch.ClientSecret
		if v == "" {
			secretEnc = ""
		} else {
			enc, err := s.Box.EncryptString(v)
			if err != nil {
				return nil, err
			}
			secretEnc = enc
		}
	}
	if enabled {
		if clientID == "" || secretEnc == "" {
			return nil, fmt.Errorf("%w: client_id and client_secret required to enable", ErrConflict)
		}
		switch provider {
		case "azure":
			if tenant == "" {
				return nil, fmt.Errorf("%w: tenant required for azure", ErrConflict)
			}
		case "authentik", "clerk":
			if baseURL == "" {
				return nil, fmt.Errorf("%w: base_url required for %s", ErrConflict, provider)
			}
		}
	}
	err = s.Pool.QueryRow(ctx, `
		UPDATE oauth_settings SET
			enabled=$2, client_id=$3, client_secret_enc=$4, redirect_uri=$5, tenant=$6, base_url=$7, updated_at=NOW()
		WHERE provider=$1
		RETURNING id, provider, enabled, client_id, client_secret_enc, redirect_uri, tenant, base_url, updated_at
	`, provider, enabled, clientID, secretEnc, redirect, tenant, baseURL).Scan(
		&id, &provider, &enabled, &clientID, &secretEnc, &redirect, &tenant, &baseURL, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &OauthSetting{
		ID: id, Provider: provider, Enabled: enabled, ClientID: clientID,
		ClientSecretSet: secretEnc != "", RedirectURI: redirect, Tenant: tenant, BaseURL: baseURL, UpdatedAt: updatedAt,
	}, nil
}
