package database

import (
	"fmt"

	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
	"golang.org/x/crypto/ssh"
)

// Start launches a managed database container on the remote host.
func Start(client *ssh.Client, db *store.Database, network, password string) error {
	if err := sshx.EnsureNetwork(client, network); err != nil {
		return err
	}
	name := "goolify-db-" + db.ID.String()
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name)

	args := []string{
		"docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", network,
	}
	args = append(args, engineEnv(db.Engine, password)...)
	if db.IsPublic && db.PublicPort != nil {
		args = append(args, "-p", fmt.Sprintf("%d:%d", *db.PublicPort, defaultPort(db.Engine)))
	}
	args = append(args, db.Image)

	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("start database: %v %s", err, errOut)
	}
	return nil
}

func Stop(client *ssh.Client, dbID string) error {
	name := "goolify-db-" + dbID
	_, errOut, err := sshx.RunArgs(client, "docker", "rm", "-f", name)
	if err != nil {
		return fmt.Errorf("stop database: %v %s", err, errOut)
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
		return []string{"-e", "POSTGRES_PASSWORD=" + password, "-e", "POSTGRES_USER=goolify", "-e", "POSTGRES_DB=goolify"}
	case "mysql", "mariadb":
		return []string{"-e", "MYSQL_ROOT_PASSWORD=" + password, "-e", "MYSQL_DATABASE=goolify"}
	case "mongodb":
		return []string{"-e", "MONGO_INITDB_ROOT_USERNAME=goolify", "-e", "MONGO_INITDB_ROOT_PASSWORD=" + password}
	case "redis", "keydb", "dragonfly":
		if password != "" {
			return []string{"-e", "REDIS_PASSWORD=" + password}
		}
		return nil
	default:
		return nil
	}
}
