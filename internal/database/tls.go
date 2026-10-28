package database

import (
	"fmt"
	"path"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// TLSMaterial is the CA and server certificate written onto the database host.
// The private key is mounted into the container and must not be logged.
type TLSMaterial struct {
	CACert []byte
	Cert   []byte
	Key    []byte
}

// SSLSupported reports engines that can mount a Dockfin server certificate.
func SSLSupported(engine string) bool {
	switch engine {
	case "postgresql", "mysql", "mariadb", "mongodb", "redis", "keydb", "dragonfly":
		return true
	default:
		return false
	}
}

// sslDir is outside the data volume. PostgreSQL and MySQL refuse to initialize
// when the data directory already contains files, and a key left inside that
// directory would be copied by every backup.
func sslDir(dbID string) string {
	return path.Join("/data/dockfin/database-ssl", dbID)
}

// RemoveTLS deletes the certificate files for a database.
func RemoveTLS(client *ssh.Client, dbID string) error {
	return runRemote(client, "remove ssl dir", "rm", "-rf", sslDir(dbID))
}

func installTLS(client *ssh.Client, dbID, engine, image string, tls *TLSMaterial) error {
	if tls == nil {
		return RemoveTLS(client, dbID)
	}
	dir := sslDir(dbID)
	if err := runRemote(client, "mkdir ssl dir", "mkdir", "-p", dir); err != nil {
		return err
	}
	files := map[string][]byte{
		path.Join(dir, "dockfin-ca.crt"): tls.CACert,
		path.Join(dir, "server.crt"):     tls.Cert,
		path.Join(dir, "server.key"):     tls.Key,
	}
	if engine == "mongodb" {
		combined := append(append([]byte{}, tls.Cert...), tls.Key...)
		if len(tls.Cert) > 0 && tls.Cert[len(tls.Cert)-1] != '\n' {
			combined = append(append([]byte{}, tls.Cert...), '\n')
			combined = append(combined, tls.Key...)
		}
		files[path.Join(dir, "server.pem")] = combined
	}
	for name, body := range files {
		if err := sshx.WriteFile(client, name, body); err != nil {
			return err
		}
	}
	if err := runRemote(client, "chmod ssl certs", "chmod", "644", path.Join(dir, "dockfin-ca.crt"), path.Join(dir, "server.crt")); err != nil {
		return err
	}
	if err := runRemote(client, "chmod ssl key", "chmod", "600", path.Join(dir, "server.key")); err != nil {
		return err
	}
	if engine == "mongodb" {
		if err := runRemote(client, "chmod mongo pem", "chmod", "600", path.Join(dir, "server.pem")); err != nil {
			return err
		}
	}
	return ownTLSFiles(client, image, engine, dir)
}

// ownTLSFiles gives the key to the user the image actually runs as.
// postgres:16-alpine is uid 70; the Debian image is uid 999. A host chown of
// 999 makes Alpine PostgreSQL reject the key.
func ownTLSFiles(client *ssh.Client, image, engine, dir string) error {
	keyFiles := []string{"server.key", "server.crt"}
	if engine == "mongodb" {
		keyFiles = append(keyFiles, "server.pem")
	}
	if owner := tlsFileOwner(engine); owner != "" {
		return chownTLSInImage(client, image, owner, dir, keyFiles)
	}
	// KeyDB and Dragonfly run as root or uid 999. Root can read a 999-owned key.
	args := []string{"chown", "999:999"}
	for _, name := range keyFiles {
		args = append(args, path.Join(dir, name))
	}
	return runRemote(client, "chown ssl files", args...)
}

func tlsFileOwner(engine string) string {
	switch engine {
	case "postgresql":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "mongodb":
		return "mongodb"
	case "redis":
		return "redis"
	default:
		return ""
	}
}

func chownTLSInImage(client *ssh.Client, image, owner, dir string, files []string) error {
	if image == "" {
		return fmt.Errorf("database image is required to set ssl file ownership")
	}
	args := []string{
		"docker", "run", "--rm", "--network", "none",
		"--user", "root",
		"--entrypoint", "/bin/chown",
		"-v", dir + ":/dockfin-certs",
		image,
		owner + ":" + owner,
	}
	for _, name := range files {
		args = append(args, "/dockfin-certs/"+name)
	}
	return runRemote(client, "chown ssl files", args...)
}

func runRemote(client *ssh.Client, what string, args ...string) error {
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("%s: %v %s", what, err, errOut)
	}
	return nil
}

func sslVolumeArgs(dbID, engine string) []string {
	dir := sslDir(dbID)
	mount := func(file, target string) []string {
		return []string{"-v", dir + "/" + file + ":" + target + ":ro"}
	}
	switch engine {
	case "postgresql":
		return append(mount("server.crt", "/var/lib/postgresql/certs/server.crt"), mount("server.key", "/var/lib/postgresql/certs/server.key")...)
	case "mysql", "mariadb":
		args := mount("server.crt", "/etc/mysql/certs/server.crt")
		args = append(args, mount("server.key", "/etc/mysql/certs/server.key")...)
		args = append(args, mount("dockfin-ca.crt", "/etc/mysql/certs/dockfin-ca.crt")...)
		return args
	case "redis":
		args := mount("server.crt", "/etc/redis/certs/server.crt")
		args = append(args, mount("server.key", "/etc/redis/certs/server.key")...)
		args = append(args, mount("dockfin-ca.crt", "/etc/redis/certs/dockfin-ca.crt")...)
		return args
	case "keydb":
		args := mount("server.crt", "/etc/keydb/certs/server.crt")
		args = append(args, mount("server.key", "/etc/keydb/certs/server.key")...)
		args = append(args, mount("dockfin-ca.crt", "/etc/keydb/certs/dockfin-ca.crt")...)
		return args
	case "dragonfly":
		args := mount("server.crt", "/etc/dragonfly/certs/server.crt")
		args = append(args, mount("server.key", "/etc/dragonfly/certs/server.key")...)
		args = append(args, mount("dockfin-ca.crt", "/etc/dragonfly/certs/dockfin-ca.crt")...)
		return args
	case "mongodb":
		args := mount("server.pem", "/etc/mongo/certs/server.pem")
		args = append(args, mount("dockfin-ca.crt", "/etc/mongo/certs/ca.pem")...)
		return args
	default:
		return nil
	}
}

func sslCommand(engine string) []string {
	switch engine {
	case "postgresql":
		return []string{
			"postgres",
			"-c", "ssl=on",
			"-c", "ssl_cert_file=/var/lib/postgresql/certs/server.crt",
			"-c", "ssl_key_file=/var/lib/postgresql/certs/server.key",
		}
	case "mysql":
		return mysqlSSLCommand("mysqld")
	case "mariadb":
		return mysqlSSLCommand("mariadbd")
	case "mongodb":
		return []string{
			"mongod",
			"--tlsMode=allowTLS",
			"--tlsAllowConnectionsWithoutCertificates",
			"--tlsAllowInvalidHostnames",
			"--tlsCAFile", "/etc/mongo/certs/ca.pem",
			"--tlsCertificateKeyFile", "/etc/mongo/certs/server.pem",
		}
	case "redis":
		return redisSSLFlags("/etc/redis/certs")
	case "keydb":
		return redisSSLFlags("/etc/keydb/certs")
	case "dragonfly":
		return []string{
			"--tls",
			"--tls_cert_file", "/etc/dragonfly/certs/server.crt",
			"--tls_key_file", "/etc/dragonfly/certs/server.key",
			"--tls_ca_cert_file", "/etc/dragonfly/certs/dockfin-ca.crt",
		}
	default:
		return nil
	}
}

func mysqlSSLCommand(bin string) []string {
	return []string{
		bin,
		"--ssl-cert=/etc/mysql/certs/server.crt",
		"--ssl-key=/etc/mysql/certs/server.key",
		"--ssl-ca=/etc/mysql/certs/dockfin-ca.crt",
		"--require-secure-transport=ON",
	}
}

func redisSSLFlags(dir string) []string {
	return []string{
		"--tls-port", "6380",
		"--tls-cert-file", dir + "/server.crt",
		"--tls-key-file", dir + "/server.key",
		"--tls-ca-cert-file", dir + "/dockfin-ca.crt",
		"--tls-auth-clients", "optional",
	}
}
