// Package database starts and stops managed database containers on remote Docker
// hosts over SSH. The control-plane Postgres pool is in package db.
package database

import (
	"fmt"
	"path"
	"strings"

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

// EngineDataPath is the in-container directory that must persist across recreates.
func EngineDataPath(engine string) string {
	return engineDataPath(engine)
}

// CoversDataDir reports whether a container path is the data directory, inside it, or a parent of it.
func CoversDataDir(engine, mount string) bool {
	data := path.Clean(engineDataPath(engine))
	mount = path.Clean(mount)
	if mount == "/" || data == "/" {
		return true
	}
	return mount == data || strings.HasPrefix(mount, data+"/") || strings.HasPrefix(data, mount+"/")
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
	case "sqlite":
		return sqliteDataDir
	default:
		return "/data"
	}
}

// VolumeArgs returns docker run -v flags for durable storage (exported for tests).
func VolumeArgs(dbID, engine string) []string {
	return []string{"-v", HostDataDir(dbID) + ":" + engineDataPath(engine)}
}

// Start launches a managed database container on the remote host with a durable bind mount.
// BindMount is an extra host file or directory mounted into the database container.
type BindMount struct {
	Host      string
	Container string
}

func Start(client *ssh.Client, db *store.Database, network, password string, extraEnv []string, tls *TLSMaterial, mounts []BindMount, scripts []InitScript) error {
	var sqliteFiles string
	if db.Engine == "sqlite" {
		if db.IsPublic {
			return fmt.Errorf("sqlite is a file and is not published on a port")
		}
		files, err := SqliteDatabases(db.EngineConfig)
		if err != nil {
			return err
		}
		sqliteFiles = files
	}
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

	if db.EnableSSL && tls == nil {
		return fmt.Errorf("database ssl is enabled but no certificate was provided")
	}
	if db.EnableSSL && !SSLSupported(db.Engine) {
		return fmt.Errorf("ssl is not supported for %s", db.Engine)
	}
	image := db.Image
	if image == "" {
		image = store.DefaultDBImage(db.Engine)
	}
	if err := installTLS(client, db.ID.String(), db.Engine, image, tls); err != nil {
		return err
	}
	scripts, err := NormalizeInitScripts(db.Engine, scripts)
	if err != nil {
		return err
	}
	if err := InstallInitScripts(client, db.ID.String(), db.Engine, scripts); err != nil {
		return err
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
	if tls != nil {
		args = append(args, sslVolumeArgs(db.ID.String(), db.Engine)...)
	}
	for _, m := range mounts {
		if CoversDataDir(db.Engine, m.Container) {
			return fmt.Errorf("mount path %s covers the database data directory", m.Container)
		}
		args = append(args, "-v", m.Host+":"+m.Container)
	}
	args = append(args, InitVolumeArgs(db.ID.String(), scripts)...)
	// User env first, then engine credentials. Docker keeps the last -e, so a
	// remote secret cannot replace the password Dockfin stored for this database.
	args = append(args, extraEnv...)
	args = append(args, engineEnv(db.Engine, password)...)
	if db.Engine == "sqlite" {
		args = append(args,
			"-e", "SQLITE_DATABASES="+sqliteFiles,
			"--health-cmd", "healthcheck.sh",
			"--health-interval", "15s",
			"--health-timeout", "5s",
			"--health-retries", "5",
			"--health-start-period", "5s",
		)
	}
	if db.IsPublic && db.PublicPort != nil {
		args = append(args, "-p", fmt.Sprintf("%d:%d", *db.PublicPort, defaultPort(db.Engine)))
	}
	imageArg := db.Image
	if imageArg == "" {
		imageArg = image
	}
	args = append(args, imageArg)
	cmd := engineCmd(db.Engine, password)
	if tls != nil {
		cmd = appendSSLCommand(db.Engine, cmd)
	}
	args = append(args, cmd...)

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
	return runRemote(client, "remove database data", "rm", "-rf", HostDataDir(dbID))
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

func appendSSLCommand(engine string, cmd []string) []string {
	extra := sslCommand(engine)
	if len(extra) == 0 {
		return cmd
	}
	switch engine {
	case "redis", "keydb", "dragonfly":
		return append(cmd, extra...)
	default:
		return extra
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
