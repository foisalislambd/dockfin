package store

import (
	"context"
	"time"

	"github.com/dockfin/dockfin/internal/traffic"
	"github.com/google/uuid"
)

// TrafficReport is the request totals shown for a server or one resource.
type TrafficReport struct {
	Requests int64              `json:"requests"`
	Series   []TrafficSeries    `json:"series"`
	Hosts    []TrafficCount     `json:"hosts"`
	Paths    []TrafficPathCount `json:"paths"`
	Statuses []TrafficCount     `json:"statuses"`
	Clients  []TrafficCount     `json:"clients"`
}

type TrafficSeries struct {
	Bucket   time.Time `json:"bucket"`
	Class    string    `json:"class"`
	Requests int64     `json:"requests"`
}

type TrafficCount struct {
	Name     string `json:"name"`
	Requests int64  `json:"requests"`
}

type TrafficPathCount struct {
	Host     string `json:"host"`
	Path     string `json:"path"`
	Requests int64  `json:"requests"`
}

func (s *Store) AddTraffic(ctx context.Context, serverID uuid.UUID, now time.Time, rows []traffic.Counted, mode string, retentionDays int) error {
	if retentionDays < 1 {
		retentionDays = 7
	}
	now = now.UTC()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, row := range rows {
		if row.N <= 0 || row.Host == "" {
			continue
		}
		bucket := row.At
		if bucket.IsZero() || bucket.Before(now.Add(-90*24*time.Hour)) || bucket.After(now.Add(5*time.Minute)) {
			bucket = now
		}
		bucket = bucket.UTC().Truncate(time.Minute)
		if _, err := tx.Exec(ctx, `
			INSERT INTO traffic_requests (server_id, bucket, host, path, method, status_class, requests)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (server_id, bucket, host, path, method, status_class)
			DO UPDATE SET requests = traffic_requests.requests + EXCLUDED.requests
		`, serverID, bucket, row.Host, row.Path, row.Method, row.Class, row.N); err != nil {
			return err
		}
		client := traffic.ApplyIPMode(row.Client, mode)
		if client == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO traffic_clients (server_id, bucket, host, client, requests)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (server_id, bucket, host, client)
			DO UPDATE SET requests = traffic_clients.requests + EXCLUDED.requests
		`, serverID, bucket, row.Host, client, row.N); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM traffic_requests
		WHERE server_id=$1 AND bucket < NOW() - make_interval(days => $2)
	`, serverID, retentionDays); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM traffic_clients
		WHERE server_id=$1 AND bucket < NOW() - make_interval(days => $2)
	`, serverID, retentionDays); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ClearTrafficClients(ctx context.Context, serverID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM traffic_clients WHERE server_id=$1`, serverID)
	return err
}

func (s *Store) TrafficReport(ctx context.Context, serverID uuid.UUID, since time.Time, hosts []string, trunc string) (*TrafficReport, error) {
	if trunc != "day" {
		trunc = "hour"
	}
	report := &TrafficReport{
		Series:   []TrafficSeries{},
		Hosts:    []TrafficCount{},
		Paths:    []TrafficPathCount{},
		Statuses: []TrafficCount{},
		Clients:  []TrafficCount{},
	}
	// A nil host list is the whole server. An empty list is a resource with no hostname.
	matchAll := hosts == nil
	if hosts == nil {
		hosts = []string{}
	}
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(requests),0) FROM traffic_requests
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
	`, serverID, since, matchAll, hosts).Scan(&report.Requests); err != nil {
		return nil, err
	}
	series, err := s.Pool.Query(ctx, `
		SELECT date_trunc($5, bucket), status_class, SUM(requests)
		FROM traffic_requests
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
		GROUP BY 1, 2 ORDER BY 1
	`, serverID, since, matchAll, hosts, trunc)
	if err != nil {
		return nil, err
	}
	defer series.Close()
	for series.Next() {
		var row TrafficSeries
		if err := series.Scan(&row.Bucket, &row.Class, &row.Requests); err != nil {
			return nil, err
		}
		report.Series = append(report.Series, row)
	}
	if err := series.Err(); err != nil {
		return nil, err
	}
	if err := s.scanCounts(ctx, &report.Hosts, `
		SELECT host, SUM(requests) FROM traffic_requests
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
		GROUP BY host ORDER BY SUM(requests) DESC LIMIT 20
	`, serverID, since, matchAll, hosts); err != nil {
		return nil, err
	}
	paths, err := s.Pool.Query(ctx, `
		SELECT host, path, SUM(requests) FROM traffic_requests
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
		GROUP BY host, path ORDER BY SUM(requests) DESC LIMIT 20
	`, serverID, since, matchAll, hosts)
	if err != nil {
		return nil, err
	}
	defer paths.Close()
	for paths.Next() {
		var row TrafficPathCount
		if err := paths.Scan(&row.Host, &row.Path, &row.Requests); err != nil {
			return nil, err
		}
		report.Paths = append(report.Paths, row)
	}
	if err := paths.Err(); err != nil {
		return nil, err
	}
	if err := s.scanCounts(ctx, &report.Statuses, `
		SELECT status_class, SUM(requests) FROM traffic_requests
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
		GROUP BY status_class ORDER BY SUM(requests) DESC
	`, serverID, since, matchAll, hosts); err != nil {
		return nil, err
	}
	if err := s.scanCounts(ctx, &report.Clients, `
		SELECT client, SUM(requests) FROM traffic_clients
		WHERE server_id=$1 AND bucket >= $2 AND ($3::bool OR host = ANY($4::text[]))
		GROUP BY client ORDER BY SUM(requests) DESC LIMIT 20
	`, serverID, since, matchAll, hosts); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Store) scanCounts(ctx context.Context, dest *[]TrafficCount, query string, serverID uuid.UUID, since time.Time, matchAll bool, hosts []string) error {
	rows, err := s.Pool.Query(ctx, query, serverID, since, matchAll, hosts)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row TrafficCount
		if err := rows.Scan(&row.Name, &row.Requests); err != nil {
			return err
		}
		*dest = append(*dest, row)
	}
	return rows.Err()
}

func (s *Store) ApplicationHosts(ctx context.Context, teamID, appID uuid.UUID) ([]string, uuid.UUID, error) {
	var fqdn string
	var dest *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(fqdn,''), destination_id FROM applications WHERE id=$1 AND team_id=$2
	`, appID, teamID).Scan(&fqdn, &dest)
	if err != nil {
		return nil, uuid.Nil, err
	}
	hosts := traffic.SplitHosts(fqdn)
	rows, err := s.Pool.Query(ctx, `
		SELECT COALESCE(fqdn,'') FROM application_previews WHERE application_id=$1 AND team_id=$2
	`, appID, teamID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var preview string
		if err := rows.Scan(&preview); err != nil {
			return nil, uuid.Nil, err
		}
		hosts = append(hosts, traffic.SplitHosts(preview)...)
	}
	if err := rows.Err(); err != nil {
		return nil, uuid.Nil, err
	}
	serverID, err := s.serverForDestination(ctx, teamID, dest)
	return uniqueHosts(hosts), serverID, err
}

func (s *Store) ServiceHosts(ctx context.Context, teamID, serviceID uuid.UUID) ([]string, uuid.UUID, error) {
	var fqdn string
	var serverID *uuid.UUID
	var dest *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(fqdn,''), server_id, destination_id FROM services WHERE id=$1 AND team_id=$2
	`, serviceID, teamID).Scan(&fqdn, &serverID, &dest)
	if err != nil {
		return nil, uuid.Nil, err
	}
	hosts := traffic.SplitHosts(fqdn)
	rows, err := s.Pool.Query(ctx, `
		SELECT host FROM service_domains WHERE service_id=$1 AND team_id=$2
	`, serviceID, teamID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return nil, uuid.Nil, err
		}
		hosts = append(hosts, traffic.SplitHosts(host)...)
	}
	if err := rows.Err(); err != nil {
		return nil, uuid.Nil, err
	}
	if serverID == nil {
		id, err := s.serverForDestination(ctx, teamID, dest)
		return uniqueHosts(hosts), id, err
	}
	return uniqueHosts(hosts), *serverID, nil
}

func (s *Store) serverForDestination(ctx context.Context, teamID uuid.UUID, dest *uuid.UUID) (uuid.UUID, error) {
	if dest == nil {
		return uuid.Nil, nil
	}
	row, err := s.GetDestination(ctx, teamID, *dest)
	if err != nil {
		return uuid.Nil, err
	}
	return row.ServerID, nil
}

func uniqueHosts(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, host := range in {
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}
