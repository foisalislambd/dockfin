package imagescan

import (
	"regexp"
	"strconv"
	"strings"
)

// TagChoice is the newer tags Coolify would offer for one current tag.
// Patch is the newest same major.minor. Line is the newest same major.
// Major is informational and is not applied by the upgrade action.
type TagChoice struct {
	Patch    string `json:"patch,omitempty"`
	Line     string `json:"line,omitempty"`
	Major    string `json:"major,omitempty"`
	Floating bool   `json:"floating"`
}

var preRelease = regexp.MustCompile(`(?i)(?:^|[._-])(?:rc|beta|alpha|dev|nightly|edge|preview|canary)\d*(?:$|[._-])`)
var versionTag = regexp.MustCompile(`^([^0-9]*)([0-9]{1,9}(?:\.[0-9]{1,9})*)((?:[-_+].*)?)$`)
var validTag = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)

type parsedTag struct {
	prefix string
	parts  []int
	suffix string
}

// SelectTags picks newer version tags that keep the current prefix and suffix.
// A tag that is not a version (latest, main, a git sha) has no candidates.
func SelectTags(current string, tags []string) TagChoice {
	cur := parseTag(current)
	if cur == nil {
		return TagChoice{}
	}
	same := map[string][]int{}
	limit := len(tags)
	if limit > 10000 {
		limit = 10000
	}
	for _, tag := range tags[:limit] {
		p := parseTag(tag)
		if p != nil && p.prefix == cur.prefix && p.suffix == cur.suffix {
			same[tag] = p.parts
		}
	}
	floating := false
	for _, parts := range same {
		if len(parts) > len(cur.parts) && samePrefix(parts, cur.parts) {
			floating = true
			break
		}
	}
	sameShape := map[string][]int{}
	for tag, parts := range same {
		if len(parts) == len(cur.parts) {
			sameShape[tag] = parts
		}
	}
	line := newest(filter(sameShape, func(parts []int) bool {
		return parts[0] == cur.parts[0] && compareParts(parts, cur.parts) >= 0
	}))
	if line == current && !floating {
		line = ""
	}
	patch := ""
	if !floating && len(cur.parts) >= 3 {
		patch = newest(filter(sameShape, func(parts []int) bool {
			return parts[0] == cur.parts[0] && parts[1] == cur.parts[1] && compareParts(parts, cur.parts) > 0
		}))
		if patch == line {
			patch = ""
		}
	}
	major := newest(filter(sameShape, func(parts []int) bool { return parts[0] > cur.parts[0] }))
	if major == "" {
		major = newest(filter(same, func(parts []int) bool { return parts[0] > cur.parts[0] }))
	}
	return TagChoice{Patch: patch, Line: line, Major: major, Floating: floating}
}

func parseTag(tag string) *parsedTag {
	if !validTag.MatchString(tag) || strings.HasPrefix(tag, "sha256-") || strings.HasSuffix(tag, ".sig") || strings.HasSuffix(tag, ".att") || strings.HasSuffix(tag, ".sbom") {
		return nil
	}
	if preRelease.MatchString(tag) {
		return nil
	}
	m := versionTag.FindStringSubmatch(tag)
	if m == nil {
		return nil
	}
	prefix := m[1]
	if prefix != "" {
		last := prefix[len(prefix)-1]
		if last != '-' && last != '_' && last != '.' && last != 'v' && last != 'V' {
			return nil
		}
		if strings.EqualFold(prefix, "sha") || strings.EqualFold(prefix, "sha-") || strings.EqualFold(prefix, "sha_") {
			return nil
		}
	}
	raw := strings.Split(m[2], ".")
	parts := make([]int, len(raw))
	for i, p := range raw {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		parts[i] = n
	}
	return &parsedTag{prefix: prefix, parts: parts, suffix: m[3]}
}

func samePrefix(parts, prefix []int) bool {
	if len(parts) < len(prefix) {
		return false
	}
	for i := range prefix {
		if parts[i] != prefix[i] {
			return false
		}
	}
	return true
}

func filter(in map[string][]int, ok func([]int) bool) map[string][]int {
	out := map[string][]int{}
	for tag, parts := range in {
		if ok(parts) {
			out[tag] = parts
		}
	}
	return out
}

func newest(tags map[string][]int) string {
	var best string
	for tag, parts := range tags {
		if best == "" || compareParts(parts, tags[best]) > 0 {
			best = tag
		}
	}
	return best
}

func compareParts(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// ImageName splits a reference into the registry repository and the tag.
// docker.io library images are normalized. A digest reference has no tag.
func ImageName(ref string) (repository, tag string, ok bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "@") || !ValidImageRef(ref) {
		return "", "", false
	}
	tag = "latest"
	name := ref
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		name = ref[:i]
		tag = ref[i+1:]
	}
	if !strings.Contains(name, "/") {
		name = "docker.io/library/" + name
	} else if !strings.Contains(strings.Split(name, "/")[0], ".") && !strings.Contains(strings.Split(name, "/")[0], ":") && strings.Split(name, "/")[0] != "localhost" {
		name = "docker.io/" + name
	}
	return name, tag, true
}
