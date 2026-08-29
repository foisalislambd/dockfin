package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PrivateKey struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	InUse       bool      `json:"in_use"`
	CreatedAt   time.Time `json:"created_at"`
}

type Server struct {
	ID                 uuid.UUID  `json:"id"`
	TeamID             uuid.UUID  `json:"team_id"`
	PrivateKeyID       *uuid.UUID `json:"private_key_id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	IP                 string     `json:"ip"`
	PublicIP           string     `json:"public_ip"`
	Port               int        `json:"port"`
	UserName           string     `json:"user_name"`
	IsReachable        bool       `json:"is_reachable"`
	IsUsable           bool       `json:"is_usable"`
	DockerVersion      string     `json:"docker_version"`
	ProxyType          string     `json:"proxy_type"`
	ProxyStatus        string     `json:"proxy_status"`
	HostKeyFingerprint string     `json:"host_key_fingerprint"`
	HostKeyType        string     `json:"host_key_type"`
	IsBuildServer      bool       `json:"is_build_server"`
	IsSwarmManager     bool       `json:"is_swarm_manager"`
	WildcardDomain     string     `json:"wildcard_domain"`
	MagicDomain        string     `json:"magic_domain"`
	JumpHostID         *uuid.UUID `json:"jump_host_id"`
	CreatedAt          time.Time  `json:"created_at"`
}

type Destination struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	ServerID  uuid.UUID `json:"server_id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Network   string    `json:"network"`
	CreatedAt time.Time `json:"created_at"`
}

const serverCols = `id, team_id, private_key_id, name, description, ip, COALESCE(public_ip,''), port, user_name,
	is_reachable, is_usable, docker_version, proxy_type, proxy_status,
	COALESCE(host_key_fingerprint,''), COALESCE(host_key_type,''), jump_host_id, created_at`

func scanServer(scan func(dest ...any) error) (*Server, error) {
	var srv Server
	err := scan(
		&srv.ID, &srv.TeamID, &srv.PrivateKeyID, &srv.Name, &srv.Description, &srv.IP, &srv.PublicIP, &srv.Port, &srv.UserName,
		&srv.IsReachable, &srv.IsUsable, &srv.DockerVersion, &srv.ProxyType, &srv.ProxyStatus,
		&srv.HostKeyFingerprint, &srv.HostKeyType, &srv.JumpHostID, &srv.CreatedAt,
	)
	return &srv, err
}

func (s *Store) CreatePrivateKey(ctx context.Context, teamID uuid.UUID, name, desc, pub, privEnc, fp string) (*PrivateKey, error) {
	var k PrivateKey
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO private_keys (team_id, name, description, public_key, private_key_enc, fingerprint)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, team_id, name, description, public_key, fingerprint, created_at
	`, teamID, name, desc, pub, privEnc, fp).Scan(
		&k.ID, &k.TeamID, &k.Name, &k.Description, &k.PublicKey, &k.Fingerprint, &k.CreatedAt,
	)
	return &k, err
}

func (s *Store) ListPrivateKeys(ctx context.Context, teamID uuid.UUID) ([]PrivateKey, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT pk.id, pk.team_id, pk.name, pk.description, pk.public_key, pk.fingerprint, pk.created_at,
			EXISTS(SELECT 1 FROM servers srv WHERE srv.private_key_id = pk.id) AS in_use
		FROM private_keys pk WHERE pk.team_id = $1 ORDER BY pk.name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateKey
	for rows.Next() {
		var k PrivateKey
		if err := rows.Scan(&k.ID, &k.TeamID, &k.Name, &k.Description, &k.PublicKey, &k.Fingerprint, &k.CreatedAt, &k.InUse); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetPrivateKey(ctx context.Context, teamID, id uuid.UUID) (*PrivateKey, error) {
	var k PrivateKey
	err := s.Pool.QueryRow(ctx, `
		SELECT pk.id, pk.team_id, pk.name, pk.description, pk.public_key, pk.fingerprint, pk.created_at,
			EXISTS(SELECT 1 FROM servers srv WHERE srv.private_key_id = pk.id) AS in_use
		FROM private_keys pk WHERE pk.id=$1 AND pk.team_id=$2
	`, id, teamID).Scan(&k.ID, &k.TeamID, &k.Name, &k.Description, &k.PublicKey, &k.Fingerprint, &k.CreatedAt, &k.InUse)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &k, err
}

func (s *Store) GetPrivateKeyMaterial(ctx context.Context, teamID, id uuid.UUID) (privEnc string, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT private_key_enc FROM private_keys WHERE id = $1 AND team_id = $2
	`, id, teamID).Scan(&privEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return privEnc, err
}

func (s *Store) UpdatePrivateKey(ctx context.Context, teamID, id uuid.UUID, name, description string) (*PrivateKey, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE private_keys SET name=$3, description=$4, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, name, description)
	if err != nil {
		return nil, err
	}
	return s.GetPrivateKey(ctx, teamID, id)
}

func (s *Store) DeletePrivateKey(ctx context.Context, teamID, id uuid.UUID) error {
	k, err := s.GetPrivateKey(ctx, teamID, id)
	if err != nil {
		return err
	}
	if k.InUse {
		return ErrConflict
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM private_keys WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CleanupUnusedPrivateKeys(ctx context.Context, teamID uuid.UUID) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM private_keys pk
		WHERE pk.team_id=$1
		  AND NOT EXISTS (SELECT 1 FROM servers srv WHERE srv.private_key_id = pk.id)
	`, teamID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) CreateServer(ctx context.Context, teamID uuid.UUID, keyID *uuid.UUID, name, desc, ip, user string, port int, proxyType string) (*Server, error) {
	if keyID != nil {
		if _, err := s.GetPrivateKey(ctx, teamID, *keyID); err != nil {
			return nil, err
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	publicIP := ""
	if n := strings.TrimSpace(ip); n != "" && n != "127.0.0.1" && n != "localhost" && n != "::1" {
		publicIP = n
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO servers (team_id, private_key_id, name, description, ip, public_ip, port, user_name, proxy_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+serverCols+`
	`, teamID, keyID, name, desc, ip, publicIP, port, user, proxyType)
	srv, err := scanServer(row.Scan)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO server_settings (server_id) VALUES ($1)`, srv.ID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO destinations (team_id, server_id, name, network)
		VALUES ($1, $2, 'Default', 'dockfin')
	`, teamID, srv.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.loadServerFlags(ctx, srv)
	return srv, nil
}

func (s *Store) loadServerFlags(ctx context.Context, srv *Server) {
	_ = s.Pool.QueryRow(ctx, `
		SELECT COALESCE(is_build_server,false), COALESCE(is_swarm_manager,false),
		       COALESCE(wildcard_domain,''), COALESCE(NULLIF(magic_domain,''),'sslip.io')
		FROM server_settings WHERE server_id=$1
	`, srv.ID).Scan(&srv.IsBuildServer, &srv.IsSwarmManager, &srv.WildcardDomain, &srv.MagicDomain)
}

func (s *Store) ListServers(ctx context.Context, teamID uuid.UUID) ([]Server, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+serverCols+` FROM servers WHERE team_id = $1 ORDER BY name`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		srv, err := scanServer(rows.Scan)
		if err != nil {
			return nil, err
		}
		s.loadServerFlags(ctx, srv)
		out = append(out, *srv)
	}
	return out, rows.Err()
}

func (s *Store) GetServer(ctx context.Context, teamID, id uuid.UUID) (*Server, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+serverCols+` FROM servers WHERE id = $1 AND team_id = $2`, id, teamID)
	srv, err := scanServer(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.loadServerFlags(ctx, srv)
	return srv, nil
}

func (s *Store) UpdateServerStatus(ctx context.Context, id uuid.UUID, reachable, usable bool, dockerVersion, proxyStatus string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE servers SET is_reachable=$2, is_usable=$3, docker_version=$4, proxy_status=$5,
		                   last_checked_at=NOW(), updated_at=NOW()
		WHERE id=$1
	`, id, reachable, usable, dockerVersion, proxyStatus)
	return err
}

func (s *Store) UpdateServerProxyStatus(ctx context.Context, id uuid.UUID, proxyStatus string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE servers SET proxy_status=$2, updated_at=NOW() WHERE id=$1
	`, id, proxyStatus)
	return err
}

func (s *Store) UpdateServerHostKey(ctx context.Context, id uuid.UUID, fingerprint, keyType string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE servers SET host_key_fingerprint=$2, host_key_type=$3, updated_at=NOW() WHERE id=$1
	`, id, fingerprint, keyType)
	return err
}

func (s *Store) GetDeploymentQueueLimit(ctx context.Context, serverID uuid.UUID) (int, error) {
	var limit int
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(deployment_queue_limit, 25) FROM server_settings WHERE server_id=$1
	`, serverID).Scan(&limit)
	if errors.Is(err, pgx.ErrNoRows) {
		return 25, nil
	}
	return limit, err
}

func (s *Store) CountActiveDeployments(ctx context.Context, serverID uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM deployments WHERE server_id=$1 AND status IN ('queued','in_progress')
	`, serverID).Scan(&n)
	return n, err
}

func (s *Store) DeleteServer(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM servers WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListDestinations(ctx context.Context, teamID uuid.UUID, serverID *uuid.UUID) ([]Destination, error) {
	q := `
		SELECT id, team_id, server_id, name, kind, network, created_at
		FROM destinations WHERE team_id = $1`
	args := []any{teamID}
	if serverID != nil {
		q += ` AND server_id = $2`
		args = append(args, *serverID)
	}
	q += ` ORDER BY name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Destination
	for rows.Next() {
		var d Destination
		if err := rows.Scan(&d.ID, &d.TeamID, &d.ServerID, &d.Name, &d.Kind, &d.Network, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDestination(ctx context.Context, teamID, id uuid.UUID) (*Destination, error) {
	var d Destination
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, server_id, name, kind, network, created_at
		FROM destinations WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&d.ID, &d.TeamID, &d.ServerID, &d.Name, &d.Kind, &d.Network, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *Store) CreateDestination(ctx context.Context, teamID, serverID uuid.UUID, name, kind, network string) (*Destination, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	if kind == "" {
		kind = "standalone"
	}
	if network == "" {
		network = "dockfin"
	}
	var d Destination
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO destinations (team_id, server_id, name, kind, network)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, team_id, server_id, name, kind, network, created_at
	`, teamID, serverID, name, kind, network).Scan(&d.ID, &d.TeamID, &d.ServerID, &d.Name, &d.Kind, &d.Network, &d.CreatedAt)
	return &d, err
}

func (s *Store) SetServerBuildServer(ctx context.Context, teamID, serverID uuid.UUID, isBuild bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE server_settings SET is_build_server=$3, updated_at=NOW()
		WHERE server_id=$1 AND EXISTS (SELECT 1 FROM servers WHERE id=$1 AND team_id=$2)
	`, serverID, teamID, isBuild)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetServerSwarmManager(ctx context.Context, teamID, serverID uuid.UUID, isSwarm bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE server_settings SET is_swarm_manager=$3, updated_at=NOW()
		WHERE server_id=$1 AND EXISTS (SELECT 1 FROM servers WHERE id=$1 AND team_id=$2)
	`, serverID, teamID, isSwarm)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetServerPublicIP(ctx context.Context, teamID, serverID uuid.UUID, publicIP string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE servers SET public_ip=$3
		WHERE id=$1 AND team_id=$2
	`, serverID, teamID, strings.TrimSpace(publicIP))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetServerDomainSettings(ctx context.Context, teamID, serverID uuid.UUID, wildcardDomain, magicDomain string) error {
	magicDomain = strings.ToLower(strings.TrimSpace(magicDomain))
	if magicDomain != "nip.io" {
		magicDomain = "sslip.io"
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE server_settings SET wildcard_domain=$3, magic_domain=$4, updated_at=NOW()
		WHERE server_id=$1 AND EXISTS (SELECT 1 FROM servers WHERE id=$1 AND team_id=$2)
	`, serverID, teamID, strings.TrimSpace(wildcardDomain), magicDomain)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListBuildServers(ctx context.Context, teamID uuid.UUID) ([]Server, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+serverCols+`
		FROM servers s
		JOIN server_settings ss ON ss.server_id = s.id
		WHERE s.team_id=$1 AND ss.is_build_server=TRUE
		ORDER BY s.name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		srv, err := scanServer(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *srv)
	}
	return out, rows.Err()
}

// ListServersForProxyRepair returns usable servers that run a shared reverse proxy.
func (s *Store) ListServersForProxyRepair(ctx context.Context) ([]Server, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+serverCols+` FROM servers
		WHERE COALESCE(is_usable, TRUE)
		  AND lower(COALESCE(NULLIF(proxy_type, ''), 'traefik')) <> 'none'
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		srv, err := scanServer(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *srv)
	}
	return out, rows.Err()
}

func (s *Store) SetServerJumpHost(ctx context.Context, teamID, serverID uuid.UUID, jumpHostID *uuid.UUID) error {
	if jumpHostID != nil {
		if *jumpHostID == serverID {
			return fmt.Errorf("%w: jump host cannot be the server itself", ErrConflict)
		}
		jump, err := s.GetServer(ctx, teamID, *jumpHostID)
		if err != nil {
			return err
		}
		if jump.JumpHostID != nil && *jump.JumpHostID == serverID {
			return fmt.Errorf("%w: jump host would create a cycle", ErrConflict)
		}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE servers SET jump_host_id=$3, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, serverID, teamID, jumpHostID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
