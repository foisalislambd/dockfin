package services

import "strings"

// databaseImageHints are common database / cache / broker image name fragments.
// Used to hide domain inputs for infra services (Coolify isDatabaseImage parity).
var databaseImageHints = []string{
	"postgres", "postgresql", "mysql", "mariadb", "mongo", "mongodb", "redis",
	"memcached", "rabbitmq", "kafka", "elasticsearch", "opensearch", "clickhouse",
	"cockroach", "cassandra", "influxdb", "minio", "valkey", "keydb", "dragonfly",
	"neon", "timescale", "qdrant", "meilisearch", "typesense", "surrealdb",
}

// IsDatabaseImage reports whether a compose service image looks like a database/cache.
func IsDatabaseImage(image string) bool {
	img := strings.ToLower(strings.TrimSpace(image))
	if img == "" {
		return false
	}
	// Strip registry/tag for matching: ghcr.io/org/postgres:16 → postgres
	if i := strings.LastIndex(img, "/"); i >= 0 {
		img = img[i+1:]
	}
	if i := strings.Index(img, ":"); i >= 0 {
		img = img[:i]
	}
	if i := strings.Index(img, "@"); i >= 0 {
		img = img[:i]
	}
	for _, h := range databaseImageHints {
		if img == h || strings.HasPrefix(img, h+"-") || strings.HasPrefix(img, h+"_") {
			return true
		}
	}
	return false
}

// JoinBaseAndComposePath combines Coolify-style base_directory + compose location
// into a single repo-relative path with a leading slash.
func JoinBaseAndComposePath(baseDir, composeLoc string) string {
	base := strings.TrimSpace(baseDir)
	if base == "" || base == "." {
		base = "/"
	}
	loc := strings.TrimSpace(composeLoc)
	if loc == "" || loc == "auto" || loc == "auto-detect" {
		loc = ""
	}
	base = filepathSlashClean("/" + strings.Trim(base, "/"))
	if loc == "" {
		return base
	}
	loc = NormalizeComposeLocation(loc)
	if loc == "" {
		return base
	}
	if base == "/" {
		return loc
	}
	return filepathSlashClean(base + loc)
}

func filepathSlashClean(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Manual clean without importing filepath to avoid Windows drive quirks in tests —
	// still reject ..
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return "/"
	}
	return "/" + strings.Join(out, "/")
}
