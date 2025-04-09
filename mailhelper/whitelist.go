package mailhelper

import (
	"strings"
)

// Config is the struct that matches the expected YAML format.
type Config struct {
	Domains []string `yaml:"domains"`
}

// IsWhitelistedEmail returns true if the email is allowed based on the allowed list.
// Allowed entries can be full addresses (e.g. "john.doe@gmail.com"), plain domains
// (e.g. "hotmail.com") or wildcard domains (e.g. "*.apple.com").
// Note: A wildcard entry like "*.apple.com" matches subdomains (e.g. "foo.apple.com")
// but does not match "apple.com" itself.
func IsWhitelistedEmail(email string, allowed []string) bool {
	// Split the email into local part and domain.
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false // invalid email address format
	}
	domain := parts[1]

	// Iterate through each allowed entry.
	for _, a := range allowed {
		// If the allowed entry is an exact match for the whole email address.
		if strings.EqualFold(email, a) {
			return true
		}
		// If the allowed entry does not contain an "@" then treat it as a domain.
		if !strings.Contains(a, "@") {
			// Check for a wildcard domain.
			if strings.HasPrefix(a, "*.") {
				// Trim the "*." prefix to get the base domain.
				baseDomain := strings.TrimPrefix(a, "*.")
				// Check that the email's domain ends with the base domain preceded by a dot.
				if strings.HasSuffix(domain, "."+baseDomain) {
					return true
				}
			} else {
				// Otherwise, check if the email domain exactly matches the allowed domain.
				if strings.EqualFold(domain, a) {
					return true
				}
			}
		}
	}
	return false
}
