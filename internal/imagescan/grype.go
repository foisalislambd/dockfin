package imagescan

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Finding is one vulnerability from a Grype report.
type Finding struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"`
	Package      string `json:"package"`
	Version      string `json:"version"`
	FixedVersion string `json:"fixed_version,omitempty"`
	Description  string `json:"description,omitempty"`
}

// Summary counts findings after ignores are removed.
type Summary struct {
	Critical int       `json:"critical"`
	High     int       `json:"high"`
	Medium   int       `json:"medium"`
	Low      int       `json:"low"`
	Unknown  int       `json:"unknown"`
	Findings []Finding `json:"findings"`
}

const maxFindings = 200

// ParseGrype reads a Grype JSON document.
func ParseGrype(raw []byte, ignored map[string]struct{}) (Summary, error) {
	var doc struct {
		Matches []struct {
			Vulnerability struct {
				ID          string `json:"id"`
				Severity    string `json:"severity"`
				Description string `json:"description"`
				Fix         struct {
					Versions []string `json:"versions"`
				} `json:"fix"`
			} `json:"vulnerability"`
			Artifact struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"artifact"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Summary{}, err
	}
	out := Summary{Findings: []Finding{}}
	for _, m := range doc.Matches {
		id := strings.TrimSpace(m.Vulnerability.ID)
		if id == "" {
			continue
		}
		if _, skip := ignored[id]; skip {
			continue
		}
		f := Finding{
			ID:          id,
			Severity:    strings.ToLower(strings.TrimSpace(m.Vulnerability.Severity)),
			Package:     m.Artifact.Name,
			Version:     m.Artifact.Version,
			Description: trim(m.Vulnerability.Description, 240),
		}
		if len(m.Vulnerability.Fix.Versions) > 0 {
			f.FixedVersion = m.Vulnerability.Fix.Versions[0]
		}
		switch f.Severity {
		case "critical":
			out.Critical++
		case "high":
			out.High++
		case "medium":
			out.Medium++
		case "low":
			out.Low++
		default:
			out.Unknown++
			if f.Severity == "" {
				f.Severity = "unknown"
			}
		}
		if len(out.Findings) < maxFindings {
			out.Findings = append(out.Findings, f)
		}
	}
	return out, nil
}

func trim(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var imageRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,253}$`)

// ValidImageRef rejects shell metacharacters in an image reference.
func ValidImageRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "..") {
		return false
	}
	return imageRefPattern.MatchString(ref)
}

var composeImage = regexp.MustCompile(`(?m)^\s*image:\s*['"]?([^\s'"#]+)`)

// ComposeImages returns image references declared in a compose file.
func ComposeImages(raw string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range composeImage.FindAllStringSubmatch(raw, -1) {
		ref := strings.TrimSpace(m[1])
		if !ValidImageRef(ref) {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
		if len(out) == 8 {
			break
		}
	}
	return out
}
