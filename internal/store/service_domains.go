package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ServiceDomain is one public hostname assigned to a compose unit in a service stack.
type ServiceDomain struct {
	ID           uuid.UUID `json:"id"`
	TeamID       uuid.UUID `json:"team_id"`
	ServiceID    uuid.UUID `json:"service_id"`
	UnitName     string    `json:"unit_name"`
	Host         string    `json:"host"`
	Scheme       string    `json:"scheme"`
	InternalPort *int      `json:"internal_port,omitempty"`
	ForceHTTPS   bool      `json:"force_https"`
	Redirect     string    `json:"redirect"`
	NoIndex      bool      `json:"noindex"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func scanServiceDomain(row pgx.Row) (ServiceDomain, error) {
	var d ServiceDomain
	var port *int32
	err := row.Scan(
		&d.ID, &d.TeamID, &d.ServiceID, &d.UnitName, &d.Host, &d.Scheme, &port,
		&d.ForceHTTPS, &d.Redirect, &d.NoIndex, &d.CreatedAt, &d.UpdatedAt,
	)
	if port != nil {
		n := int(*port)
		d.InternalPort = &n
	}
	return d, err
}

const serviceDomainCols = `id, team_id, service_id, unit_name, host, scheme, internal_port, force_https, redirect, noindex, created_at, updated_at`

func (s *Store) ListServiceDomains(ctx context.Context, teamID, serviceID uuid.UUID) ([]ServiceDomain, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+serviceDomainCols+`
		FROM service_domains
		WHERE team_id=$1 AND service_id=$2
		ORDER BY unit_name, created_at
	`, teamID, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServiceDomain
	for rows.Next() {
		d, err := scanServiceDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetServiceDomain(ctx context.Context, teamID, serviceID, id uuid.UUID) (*ServiceDomain, error) {
	d, err := scanServiceDomain(s.Pool.QueryRow(ctx, `
		SELECT `+serviceDomainCols+`
		FROM service_domains WHERE id=$1 AND team_id=$2 AND service_id=$3
	`, id, teamID, serviceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

type ServiceDomainInput struct {
	UnitName     string
	Host         string
	Scheme       string
	InternalPort *int
	ForceHTTPS   bool
	Redirect     string
	NoIndex      bool
}

func (s *Store) CreateServiceDomain(ctx context.Context, teamID, serviceID uuid.UUID, in ServiceDomainInput) (*ServiceDomain, error) {
	var port *int32
	if in.InternalPort != nil {
		n := int32(*in.InternalPort)
		port = &n
	}
	d, err := scanServiceDomain(s.Pool.QueryRow(ctx, `
		INSERT INTO service_domains (team_id, service_id, unit_name, host, scheme, internal_port, force_https, redirect, noindex)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+serviceDomainCols+`
	`, teamID, serviceID, in.UnitName, in.Host, in.Scheme, port, in.ForceHTTPS, in.Redirect, in.NoIndex))
	if err != nil {
		if strings.Contains(err.Error(), "service_domains_service_host") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) UpdateServiceDomain(ctx context.Context, teamID, serviceID, id uuid.UUID, in ServiceDomainInput) (*ServiceDomain, error) {
	var port *int32
	if in.InternalPort != nil {
		n := int32(*in.InternalPort)
		port = &n
	}
	d, err := scanServiceDomain(s.Pool.QueryRow(ctx, `
		UPDATE service_domains SET
			unit_name=$4, host=$5, scheme=$6, internal_port=$7, force_https=$8, redirect=$9, noindex=$10, updated_at=NOW()
		WHERE id=$1 AND team_id=$2 AND service_id=$3
		RETURNING `+serviceDomainCols+`
	`, id, teamID, serviceID, in.UnitName, in.Host, in.Scheme, port, in.ForceHTTPS, in.Redirect, in.NoIndex))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "service_domains_service_host") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) DeleteServiceDomain(ctx context.Context, teamID, serviceID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM service_domains WHERE id=$1 AND team_id=$2 AND service_id=$3
	`, id, teamID, serviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
