package mailhelper

import "testing"

func TestIsWhitelistedEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		allowed []string
		want    bool
	}{
		{
			name:    "Exact email match",
			email:   "john.doe@gmail.com",
			allowed: []string{"john.doe@gmail.com"},
			want:    true,
		},
		{
			name:    "Exact domain match",
			email:   "john.doe@gmail.com",
			allowed: []string{"gmail.com"},
			want:    true,
		},
		{
			name:    "Non-matching domain",
			email:   "jane.doe@hotmail.com",
			allowed: []string{"gmail.com"},
			want:    false,
		},
		{
			name:    "Wildcard subdomain match",
			email:   "user@foo.apple.com",
			allowed: []string{"*.apple.com"},
			want:    true,
		},
		{
			name:    "Wildcard non-match for base domain",
			email:   "user@apple.com",
			allowed: []string{"*.apple.com"},
			want:    false,
		},
		{
			name:    "Invalid email format",
			email:   "notanemail",
			allowed: []string{"gmail.com"},
			want:    false,
		},
		{
			name:    "Case insensitive exact email match",
			email:   "Test@Gmail.com",
			allowed: []string{"test@gmail.com"},
			want:    true,
		},
		{
			name:    "Multiple allowed entries, one match",
			email:   "user@yahoo.com",
			allowed: []string{"gmail.com", "yahoo.com"},
			want:    true,
		},
		{
			name:    "Multiple allowed entries, no match",
			email:   "user@outlook.com",
			allowed: []string{"gmail.com", "yahoo.com"},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWhitelistedEmail(tc.email, tc.allowed)
			if got != tc.want {
				t.Errorf("IsWhitelistedEmail(%q, %v) = %v, want %v", tc.email, tc.allowed, got, tc.want)
			}
		})
	}
}
