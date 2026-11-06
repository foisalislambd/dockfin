package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ImageScanPolicy struct {
	Enabled bool   `json:"enabled"`
	Gate    string `json:"gate"`
}

type ImageScan struct {
	ID           uuid.UUID       `json:"id"`
	TeamID       uuid.UUID       `json:"team_id"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uuid.UUID       `json:"resource_id"`
	DeploymentID *uuid.UUID      `json:"deployment_id,omitempty"`
	ServerID     *uuid.UUID      `json:"server_id,omitempty"`
	ImageRef     string          `json:"image_ref"`
	Status       string          `json:"status"`
	Critical     int             `json:"critical"`
	High         int             `json:"high"`
	Medium       int             `json:"medium"`
	Low          int             `json:"low"`
	Unknown      int             `json:"unknown"`
	Findings     json.RawMessage `json:"findings"`
	Candidates   json.RawMessage `json:"candidates"`
	Error        string          `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
}

func (s *Store) ImageScanPolicy(ctx context.Context) (ImageScanPolicy, error) {
	var p ImageScanPolicy
	err := s.Pool.QueryRow(ctx, `SELECT image_scan_enabled, image_scan_gate FROM instance_settings WHERE id=1`).Scan(&p.Enabled, &p.Gate)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImageScanPolicy{Enabled: true, Gate: "off"}, nil
	}
	if p.Gate == "" {
		p.Gate = "off"
	}
	return p, err
}

func (s *Store) SetImageScanPolicy(ctx context.Context, enabled bool, gate string) error {
	switch gate {
	case "off", "warn", "block":
	default:
		return errors.New("gate must be off, warn, or block")
	}
	_, err := s.Pool.Exec(ctx, `UPDATE instance_settings SET image_scan_enabled=$1, image_scan_gate=$2, updated_at=NOW() WHERE id=1`, enabled, gate)
	return err
}

func (s *Store) InsertImageScan(ctx context.Context, scan ImageScan) (*ImageScan, error) {
	if len(scan.Findings) == 0 {
		scan.Findings = []byte("[]")
	}
	if scan.Status == "" {
		scan.Status = "completed"
	}
	now := time.Now().UTC()
	scan.FinishedAt = &now
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO image_scans (
			team_id, resource_type, resource_id, deployment_id, server_id, image_ref, status,
			critical, high, medium, low, unknown, findings, error, finished_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, created_at
	`, scan.TeamID, scan.ResourceType, scan.ResourceID, scan.DeploymentID, scan.ServerID, scan.ImageRef, scan.Status,
		scan.Critical, scan.High, scan.Medium, scan.Low, scan.Unknown, scan.Findings, scan.Error, scan.FinishedAt,
	).Scan(&scan.ID, &scan.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &scan, nil
}

func (s *Store) ListImageScans(ctx context.Context, teamID uuid.UUID, limit int) ([]ImageScan, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, deployment_id, server_id, image_ref, status,
			critical, high, medium, low, unknown, findings, candidates, error, created_at, finished_at
		FROM image_scans WHERE team_id=$1 ORDER BY created_at DESC LIMIT $2
	`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanImageRows(rows)
}

func (s *Store) GetImageScan(ctx context.Context, teamID, id uuid.UUID) (*ImageScan, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, deployment_id, server_id, image_ref, status,
			critical, high, medium, low, unknown, findings, candidates, error, created_at, finished_at
		FROM image_scans WHERE team_id=$1 AND id=$2
	`, teamID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanImageRows(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

func scanImageRows(rows pgx.Rows) ([]ImageScan, error) {
	var out []ImageScan
	for rows.Next() {
		var sc ImageScan
		if err := rows.Scan(
			&sc.ID, &sc.TeamID, &sc.ResourceType, &sc.ResourceID, &sc.DeploymentID, &sc.ServerID, &sc.ImageRef, &sc.Status,
			&sc.Critical, &sc.High, &sc.Medium, &sc.Low, &sc.Unknown, &sc.Findings, &sc.Candidates, &sc.Error, &sc.CreatedAt, &sc.FinishedAt,
		); err != nil {
			return nil, err
		}
		if len(sc.Findings) == 0 {
			sc.Findings = []byte("[]")
		}
		if len(sc.Candidates) == 0 {
			sc.Candidates = []byte("{}")
		}
		out = append(out, sc)
	}
	if out == nil {
		out = []ImageScan{}
	}
	return out, rows.Err()
}

func (s *Store) SetImageScanCandidates(ctx context.Context, teamID, id uuid.UUID, candidates []byte) error {
	if len(candidates) == 0 {
		candidates = []byte("{}")
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE image_scans SET candidates=$3 WHERE team_id=$1 AND id=$2`, teamID, id, candidates)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetApplicationImageTag(ctx context.Context, teamID, id uuid.UUID, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" || len(tag) > 128 || strings.ContainsAny(tag, " \t\r\n") {
		return errors.New("invalid image tag")
	}
	var pack string
	err := s.Pool.QueryRow(ctx, `SELECT build_pack FROM applications WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&pack)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if pack != "dockerimage" {
		return errors.New("only a Docker image application can change its tag here")
	}
	_, err = s.Pool.Exec(ctx, `UPDATE applications SET docker_registry_image_tag=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`, id, teamID, tag)
	return err
}

func (s *Store) ImageScanIgnores(ctx context.Context, teamID uuid.UUID) (map[string]struct{}, error) {
	rows, err := s.Pool.Query(ctx, `SELECT vuln_id FROM image_scan_ignores WHERE team_id=$1`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) IgnoreImageVuln(ctx context.Context, teamID uuid.UUID, vulnID, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO image_scan_ignores (team_id, vuln_id, reason)
		VALUES ($1,$2,$3)
		ON CONFLICT (team_id, vuln_id) DO UPDATE SET reason=EXCLUDED.reason
	`, teamID, vulnID, reason)
	return err
}

func (s *Store) DeleteImageVulnIgnore(ctx context.Context, teamID uuid.UUID, vulnID string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM image_scan_ignores WHERE team_id=$1 AND vuln_id=$2`, teamID, vulnID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
