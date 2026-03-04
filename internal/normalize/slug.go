package normalize

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumRegex = regexp.MustCompile(`[^a-z0-9]+`)
	leadTrailDash    = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a display name to a kebab-case slug.
// For example, "CA001: Require MFA for admins" becomes "ca001-require-mfa-for-admins".
func Slugify(displayName string) string {
	s := strings.ToLower(displayName)
	s = nonAlphanumRegex.ReplaceAllString(s, "-")
	s = leadTrailDash.ReplaceAllString(s, "")
	return s
}
