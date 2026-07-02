package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/pkg/utils"
)

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"simple valid", "user@example.com", true},
		{"with dot in local", "test.user@domain.org", true},
		{"short TLD", "a@b.co", true},
		{"with plus sign", "user+tag@domain.com", true},
		{"with percent", "user%name@domain.com", true},
		{"with digits", "user123@123.com", true},
		{"empty string", "", false},
		{"no at sign", "userdomain.com", false},
		{"no domain", "user@", false},
		{"no local part", "@domain.com", false},
		{"no TLD", "user@domain", false},
		{"double at", "user@@domain.com", false},
		{"TLD too short", "user@domain.c", false},
		{"special chars invalid", "user!name@domain.com", false},
		{"space in email", "user @domain.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.IsEmail(tt.email)
			assert.Equal(t, tt.want, got, "IsEmail(%q)", tt.email)
		})
	}
}
