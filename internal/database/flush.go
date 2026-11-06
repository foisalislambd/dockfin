package database

import "fmt"

// CacheFlushArgs builds a docker exec that runs FLUSHALL.
// Redis, KeyDB, and Dragonfly speak the Redis protocol. The password is passed
// as REDISCLI_AUTH so it is not a redis-cli argument.
func CacheFlushArgs(engine, container, password string) ([]string, error) {
	var bin string
	switch engine {
	case "redis", "dragonfly":
		bin = "redis-cli"
	case "keydb":
		bin = "keydb-cli"
	default:
		return nil, fmt.Errorf("cache flush is only available for redis, keydb, and dragonfly")
	}
	if container == "" {
		return nil, fmt.Errorf("container required")
	}
	args := []string{"docker", "exec"}
	if password != "" {
		args = append(args, "-e", "REDISCLI_AUTH="+password)
	}
	args = append(args, container, bin, "FLUSHALL")
	return args, nil
}
