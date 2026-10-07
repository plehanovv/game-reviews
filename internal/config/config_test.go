package config

import "testing"

func TestLoad(t *testing.T) {
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
