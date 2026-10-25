package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"default", "", "127.0.0.1:8080", false},
		{"container", ":8080", ":8080", false},
		{"ipv6", "[::1]:8081", "[::1]:8081", false},
		{"missing port", "localhost", "", true},
		{"invalid port", ":abc", "", true},
		{"out of range", ":65536", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", tc.input)
			cfg, err := Load()
			if (err != nil) != tc.invalid {
				t.Fatalf("Load error = %v", err)
			}
			if !tc.invalid && cfg.HTTPAddr != tc.want {
				t.Fatalf("got %q, want %q", cfg.HTTPAddr, tc.want)
			}
		})
	}
}

func TestCookieSecurity(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	for _, tc := range []struct {
		value           string
		secure, invalid bool
	}{
		{"", true, false}, {"true", true, false}, {"false", false, false}, {"invalid", false, true},
	} {
		t.Setenv("COOKIE_SECURE", tc.value)
		cfg, err := Load()
		if (err != nil) != tc.invalid || (!tc.invalid && cfg.CookieSecure != tc.secure) {
			t.Fatalf("COOKIE_SECURE %q: %+v, %v", tc.value, cfg, err)
		}
	}
}

func TestMissingDatabaseURL(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:8080")
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database configuration error")
	}
}
