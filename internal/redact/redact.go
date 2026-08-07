package redact

import (
	"regexp"
	"strings"
)

var (
	urlUserInfo = regexp.MustCompile(`(?i)(https?://)([^/@\s]+):([^@/\s]+)@`)
	xAccessTok  = regexp.MustCompile(`(?i)x-access-token:[^@\s/]+@`)
	oauth2Tok   = regexp.MustCompile(`(?i)oauth2:[^@\s/]+@`)
	ghTokens    = regexp.MustCompile(`(?i)\b(gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`)
	bearerHdr   = regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)(\S+)`)
)

// Secrets strips credentials commonly leaked in git/HTTP error text.
func Secrets(s string) string {
	if s == "" {
		return s
	}
	out := urlUserInfo.ReplaceAllString(s, `${1}${2}:***@`)
	out = xAccessTok.ReplaceAllString(out, "x-access-token:***@")
	out = oauth2Tok.ReplaceAllString(out, "oauth2:***@")
	out = ghTokens.ReplaceAllString(out, "***")
	out = bearerHdr.ReplaceAllString(out, `${1}***`)
	return out
}

// Error returns err.Error() with secrets redacted. Nil-safe.
func Error(err error) string {
	if err == nil {
		return ""
	}
	return Secrets(err.Error())
}

// Join redacts a multi-part message.
func Join(parts ...string) string {
	return Secrets(strings.Join(parts, " "))
}
