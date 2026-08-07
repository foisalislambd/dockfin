// Package database starts and stops managed database containers on remote Docker
// hosts over SSH. The control-plane Postgres pool is in package db.
package database

import (
	"fmt"
	"path"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

const hostDatabasesRoot = "/data/dockfin/databases"

// HostDataDir returns the durable bind-mount directory for a managed database.
func HostDataDir(dbID string) string {
	return path.Join(hostDatabasesRoot, dbID)
}

// ContainerName returns the Docker container name for a managed database.
func ContainerName(dbID string) string {
	return "dockfin-db-" + dbID
}

// engineDataPath is the in-container directory that must persist across recreates.
func engineDataPath(engine string) string {
	switch engine {
	case "postgresql":
		return "/var/lib/postgresql/data"
	case "mysql", "mariadb":
		return "/var/lib/mysql"
	case "mongodb":
		return "/data/db"
	case "redis", "keydb", "dragonfly":
		return "/data"
	case "clickhouse":
		return "/var/lib/clickhouse"
	default:
		return "/data"
	}
}

// VolumeArgs returns docker run -v flags for durable storage (exported for tests).
func VolumeArgs(dbID, engine string) []string {
	return []string{"-v", HostDataDir(dbID) + ":" + engineDataPath(engine)}
}

// Start launches a managed database container on the remote host with a durable bind mount.
func Start(client *ssh.Client, db *store.Database, network, password string) error {
	if err := sshx.EnsureNetwork(client, network); err != nil {
		return err
	}
	if err := sshx.EnsureDataDirs(client); err != nil {
		return err
	}
	hostDir := HostDataDir(db.ID.String())
	if _, errOut, err := sshx.RunArgs(client, "mkdir", "-p", hostDir); err != nil {
		return fmt.Errorf("mkdir database data dir: %v %s", err, errOut)
	}

	name := ContainerName(db.ID.String())
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name)

	args := []string{
		"docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", network,
	}
	args = append(args, VolumeArgs(db.ID.String(), db.Engine)...)
	args = append(args, engineEnv(db.Engine, password)...)
	if db.IsPublic && db.PublicPort != nil {
		args = append(args, "-p", fmt.Sprintf("%d:%d", *db.PublicPort, defaultPort(db.Engine)))
	}
	args = append(args, db.Image)
	args = append(args, engineCmd(db.Engine, password)...)

	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("start database: %v %s", err, errOut)
	}
	return nil
}

func Stop(client *ssh.Client, dbID string) error {
	name := ContainerName(dbID)
	_, errOut, err := sshx.RunArgs(client, "docker", "rm", "-f", name)
	if err != nil {
		return fmt.Errorf("stop database: %v %s", err, errOut)
	}
	return nil
}

// RemoveData deletes the durable host data directory for a managed database.
func RemoveData(client *ssh.Client, dbID string) error {
	dir := HostDataDir(dbID)
	_, errOut, err := sshx.RunArgs(client, "rm", "-rf", dir)
	if err != nil {
		return fmt.Errorf("remove database data: %v %s", err, errOut)
	}
	return nil
}

func defaultPort(engine string) int {
	switch engine {
	case "postgresql":
		return 5432
	case "mysql", "mariadb":
		return 3306
	case "mongodb":
		return 27017
	case "redis", "keydb", "dragonfly":
		return 6379
	case "clickhouse":
		return 8123
	default:
		return 0
	}
}

func engineEnv(engine, password string) []string {
	switch engine {
	case "postgresql":
		return []string{"-e", "POSTGRES_PASSWORD=" + password, "-e", "POSTGRES_USER=dockfin", "-e", "POSTGRES_DB=dockfin"}
	case "mysql", "mariadb":
		return []string{"-e", "MYSQL_ROOT_PASSWORD=" + password, "-e", "MYSQL_DATABASE=dockfin"}
	case "mongodb":
		return []string{"-e", "MONGO_INITDB_ROOT_USERNAME=dockfin", "-e", "MONGO_INITDB_ROOT_PASSWORD=" + password}
	case "redis", "keydb", "dragonfly":
		// Password is applied via engineCmd (--requirepass); official images ignore REDIS_PASSWORD alone.
		return nil
	default:
		return nil
	}
}

// engineCmd returns the container command override (after the image).
func engineCmd(engine, password string) []string {
	switch engine {
	case "redis":
		args := []string{"redis-server", "--appendonly", "yes", "--dir", "/data"}
		if password != "" {
			args = append(args, "--requirepass", password)
		}
		return args
	case "keydb":
		args := []string{"keydb-server", "--appendonly", "yes", "--dir", "/data"}
		if password != "" {
			args = append(args, "--requirepass", password)
		}
		return args
	case "dragonfly":
		// Official dragonfly image ENTRYPOINT is already "dragonfly"; only pass flags.
		args := []string{"--dir", "/data"}
		if password != "" {
			args = append(args, "--requirepass", password)
		}
		return args
	default:
		return nil
	}
}
