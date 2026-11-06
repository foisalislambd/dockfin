package containerfiles

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
)

const MaxFileBytes = 512 * 1024

// Entry is one directory child inside a container.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModUnix int64  `json:"mod_unix"`
}

// CleanPath accepts an absolute container path and rejects traversal.
func CleanPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "/"
	}
	if strings.Contains(p, "\x00") || strings.ContainsAny(p, "\n\r") {
		return "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path must be absolute")
	}
	cleaned := path.Clean(p)
	if cleaned != p && !(p == "/" && cleaned == "/") {
		// path.Clean drops a trailing slash. Allow "/tmp/" → "/tmp" except root.
		if strings.TrimRight(p, "/") != cleaned {
			return "", fmt.Errorf("invalid path")
		}
	}
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("invalid path")
	}
	return cleaned, nil
}

// ValidContainerName restricts docker exec to a single container name.
func ValidContainerName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for i, r := range name {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-'
		if !ok || (i == 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

// ListArgs lists one directory with GNU/BusyBox stat. No shell is involved.
func ListArgs(container, dir string) ([]string, error) {
	dir, err := CleanPath(dir)
	if err != nil {
		return nil, err
	}
	if !ValidContainerName(container) {
		return nil, fmt.Errorf("invalid container")
	}
	return []string{
		"docker", "exec", container,
		"find", dir, "-mindepth", "1", "-maxdepth", "1",
		"-exec", "stat", "-c", "%F|%s|%Y|%n", "{}", "+",
	}, nil
}

// ReadArgs prints a file. Callers enforce MaxFileBytes before returning it.
func ReadArgs(container, file string) ([]string, error) {
	file, err := CleanPath(file)
	if err != nil {
		return nil, err
	}
	if !ValidContainerName(container) {
		return nil, fmt.Errorf("invalid container")
	}
	return []string{"docker", "exec", container, "cat", "--", file}, nil
}

// SizeArgs prints the byte size of a file.
func SizeArgs(container, file string) ([]string, error) {
	file, err := CleanPath(file)
	if err != nil {
		return nil, err
	}
	if !ValidContainerName(container) {
		return nil, fmt.Errorf("invalid container")
	}
	return []string{"docker", "exec", container, "wc", "-c", "--", file}, nil
}

// MkdirArgs creates a directory inside the container.
func MkdirArgs(container, dir string) ([]string, error) {
	dir, err := CleanPath(dir)
	if err != nil {
		return nil, err
	}
	if dir == "/" {
		return nil, fmt.Errorf("refusing to mkdir /")
	}
	if !ValidContainerName(container) {
		return nil, fmt.Errorf("invalid container")
	}
	return []string{"docker", "exec", container, "mkdir", "-p", "--", dir}, nil
}

// RemoveFileArgs deletes one file, never a directory tree.
func RemoveFileArgs(container, file string) ([]string, error) {
	file, err := CleanPath(file)
	if err != nil {
		return nil, err
	}
	if file == "/" {
		return nil, fmt.Errorf("refusing to delete /")
	}
	if !ValidContainerName(container) {
		return nil, fmt.Errorf("invalid container")
	}
	return []string{"docker", "exec", container, "rm", "-f", "--", file}, nil
}

// ParseStatLines parses `stat -c %F|%s|%Y|%n` output.
func ParseStatLines(out string) []Entry {
	var entries []Entry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		full := parts[3]
		size, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		mod, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		entries = append(entries, Entry{
			Name:    path.Base(full),
			Path:    full,
			IsDir:   strings.Contains(strings.ToLower(parts[0]), "directory"),
			Size:    size,
			ModUnix: mod,
		})
	}
	return entries
}

// ParseByteCount reads the first integer from `wc -c` output.
func ParseByteCount(out string) (int64, bool) {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
