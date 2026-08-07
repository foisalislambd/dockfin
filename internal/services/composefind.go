package services

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CommonComposeFilenames are checked in preference order at each directory level.
var CommonComposeFilenames = []string{
	"docker-compose.yaml",
	"docker-compose.yml",
	"compose.yaml",
	"compose.yml",
}

// FindComposeFiles walks root (max depth 2 directories → files like /a/b/compose.yml)
// and returns repo-relative paths. Skips .git, node_modules, vendor.
// Depth matches remote deploy discovery (find -maxdepth 3).
func FindComposeFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == ".venv" || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			depth := 0
			if rel != "." {
				depth = strings.Count(rel, "/") + 1
			}
			// Keep in sync with deploy findComposeFilesRemote (-maxdepth 3):
			// files may live at ./a/b/file (find depth 3). Do not enter ./a/b/c/.
			if depth > 2 {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !isComposeFilename(name) {
			return nil
		}
		if rel == "." || rel == name {
			found = append(found, "/"+name)
			return nil
		}
		found = append(found, "/"+rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return SortComposeFiles(found), nil
}

// PreferComposeFile returns the best candidate, or "" if none.
func PreferComposeFile(paths []string) string {
	sorted := SortComposeFiles(paths)
	if len(sorted) == 0 {
		return ""
	}
	return sorted[0]
}

// SortComposeFiles returns paths ordered by Dockfin preference (best first).
func SortComposeFiles(paths []string) []string {
	out := uniqueStrings(append([]string{}, paths...))
	sort.SliceStable(out, func(i, j int) bool {
		return composePathRank(out[i]) < composePathRank(out[j])
	})
	return out
}

// NormalizeComposeLocation ensures a leading-slash repo path.
func NormalizeComposeLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	loc = strings.TrimPrefix(loc, "./")
	if loc == "" || loc == "auto" || loc == "auto-detect" {
		return ""
	}
	loc = filepath.ToSlash(loc)
	if !strings.HasPrefix(loc, "/") {
		loc = "/" + loc
	}
	return loc
}

func isComposeFilename(name string) bool {
	for _, n := range CommonComposeFilenames {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

func composePathRank(p string) int {
	p = strings.TrimPrefix(filepath.ToSlash(p), "/")
	dir := filepath.ToSlash(filepath.Dir(p))
	base := filepath.Base(p)
	depth := 0
	if dir != "." && dir != "" {
		depth = strings.Count(dir, "/") + 1
	}
	nameScore := 50
	for i, n := range CommonComposeFilenames {
		if strings.EqualFold(base, n) {
			nameScore = i
			break
		}
	}
	return depth*10 + nameScore
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
