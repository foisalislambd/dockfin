package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SSLCertificate is the public view of a stored certificate. The private key is never included.
type SSLCertificate struct {
	ID         uuid.UUID  `json:"id"`
	TeamID     uuid.UUID  `json:"team_id"`
	ServerID   uuid.UUID  `json:"server_id"`
	Resource   string     `json:"resource_type,omitempty"`
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`
	CommonName string     `json:"common_name"`
	SANs       []string   `json:"subject_alternative_names"`
	ValidUntil time.Time  `json:"valid_until"`
	IsCA       bool       `json:"is_ca"`
}

// SSLMaterial is a decrypted certificate and key. Callers must not serialize the key.
type SSLMaterial struct {
	SSLCertificate
	CertPEM string `json:"-"`
	KeyPEM  string `json:"-"`
}

func (s *Store) GetServerCA(ctx context.Context, teamID, serverID uuid.UUID) (*SSLMaterial, error) {
	return s.loadSSL(ctx, `
		SELECT id, team_id, server_id, resource_type, resource_id, cert_enc, key_enc,
		       common_name, subject_alternative_names, valid_until, is_ca
		FROM ssl_certificates
		WHERE team_id=$1 AND server_id=$2 AND is_ca
	`, teamID, serverID)
}

func (s *Store) GetResourceCertificate(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) (*SSLMaterial, error) {
	return s.loadSSL(ctx, `
		SELECT id, team_id, server_id, resource_type, resource_id, cert_enc, key_enc,
		       common_name, subject_alternative_names, valid_until, is_ca
		FROM ssl_certificates
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3 AND NOT is_ca
	`, teamID, resourceType, resourceID)
}

func (s *Store) SaveServerCA(ctx context.Context, teamID, serverID uuid.UUID, certEnc, keyEnc, commonName string, validUntil time.Time) (*SSLCertificate, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var row SSLCertificate
	var sans []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO ssl_certificates (
			team_id, server_id, resource_type, cert_enc, key_enc, common_name, valid_until, is_ca
		) VALUES ($1,$2,'',$3,$4,$5,$6,TRUE)
		ON CONFLICT (server_id) WHERE is_ca DO UPDATE SET
			cert_enc=EXCLUDED.cert_enc,
			key_enc=EXCLUDED.key_enc,
			common_name=EXCLUDED.common_name,
			valid_until=EXCLUDED.valid_until
		RETURNING id, team_id, server_id, common_name, subject_alternative_names, valid_until, is_ca
	`, teamID, serverID, certEnc, keyEnc, commonName, validUntil).Scan(
		&row.ID, &row.TeamID, &row.ServerID, &row.CommonName, &sans, &row.ValidUntil, &row.IsCA,
	)
	if err != nil {
		return nil, err
	}
	// A new CA must not leave leaves signed by the previous key. Doing this in
	// the same transaction means a failed delete rolls the new CA back too.
	if _, err := tx.Exec(ctx, `
		DELETE FROM ssl_certificates
		WHERE team_id=$1 AND server_id=$2 AND NOT is_ca
	`, teamID, serverID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	row.SANs = decodeSANs(sans)
	return &row, nil
}

func (s *Store) SaveResourceCertificate(ctx context.Context, teamID, serverID, resourceID uuid.UUID, resourceType, certEnc, keyEnc, commonName string, sans []string, validUntil time.Time) (*SSLCertificate, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	if sans == nil {
		sans = []string{}
	}
	raw, err := json.Marshal(sans)
	if err != nil {
		return nil, err
	}
	var row SSLCertificate
	var stored []byte
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO ssl_certificates (
			team_id, server_id, resource_type, resource_id, cert_enc, key_enc, common_name,
			subject_alternative_names, valid_until, is_ca
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,FALSE)
		ON CONFLICT (resource_type, resource_id) WHERE NOT is_ca AND resource_id IS NOT NULL DO UPDATE SET
			server_id=EXCLUDED.server_id,
			cert_enc=EXCLUDED.cert_enc,
			key_enc=EXCLUDED.key_enc,
			common_name=EXCLUDED.common_name,
			subject_alternative_names=EXCLUDED.subject_alternative_names,
			valid_until=EXCLUDED.valid_until
		RETURNING id, team_id, server_id, resource_type, resource_id, common_name, subject_alternative_names, valid_until, is_ca
	`, teamID, serverID, resourceType, resourceID, certEnc, keyEnc, commonName, raw, validUntil).Scan(
		&row.ID, &row.TeamID, &row.ServerID, &row.Resource, &row.ResourceID, &row.CommonName, &stored, &row.ValidUntil, &row.IsCA,
	)
	if err != nil {
		return nil, err
	}
	row.SANs = decodeSANs(stored)
	return &row, nil
}

func (s *Store) DeleteServerLeafCertificates(ctx context.Context, teamID, serverID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM ssl_certificates
		WHERE team_id=$1 AND server_id=$2 AND NOT is_ca
	`, teamID, serverID)
	return err
}

func (s *Store) DeleteResourceCertificate(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM ssl_certificates
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3 AND NOT is_ca
	`, teamID, resourceType, resourceID)
	return err
}

func (s *Store) loadSSL(ctx context.Context, query string, args ...any) (*SSLMaterial, error) {
	var row SSLMaterial
	var certEnc, keyEnc string
	var resourceID *uuid.UUID
	var sans []byte
	err := s.Pool.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.TeamID, &row.ServerID, &row.Resource, &resourceID, &certEnc, &keyEnc,
		&row.CommonName, &sans, &row.ValidUntil, &row.IsCA,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.ResourceID = resourceID
	row.SANs = decodeSANs(sans)
	row.CertPEM, err = s.Box.DecryptString(certEnc)
	if err != nil {
		return nil, err
	}
	row.KeyPEM, err = s.Box.DecryptString(keyEnc)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func decodeSANs(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []string{}
	}
	return out
}
