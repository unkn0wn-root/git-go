package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeSHA1(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{
			input:    []byte("Hello, World!"),
			expected: "0a0a9f2a6772942557ab5355d76af442f8f65e01",
		},
		{
			input:    []byte(""),
			expected: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
		{
			input:    []byte("test"),
			expected: "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := ComputeSHA1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeObjectHash(t *testing.T) {
	data := []byte("Hello, World!")
	objType := "blob"

	result := ComputeObjectHash(objType, data)

	assert.Len(t, result, 40)
	assert.Regexp(t, "^[0-9a-f]{40}$", result)
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		hash     string
		expected bool
	}{
		{"0a0a9f2a6772942557ab5355d76af442f8f65e01", true},
		{"da39a3ee5e6b4b0d3255bfef95601890afd80709", true},
		{"invalid", false},
		{"0a0a9f2a6772942557ab5355d76af442f8f65e0", false},
		{"0a0a9f2a6772942557ab5355d76af442f8f65e011", false},
		{"0a0a9f2a6772942557ab5355d76af442f8f65eGH", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			result := ValidateHash(tt.hash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShortHash(t *testing.T) {
	fullHash := "0a0a9f2a6772942557ab5355d76af442f8f65e01"

	tests := []struct {
		length   int
		expected string
	}{
		{7, "0a0a9f2"},
		{8, "0a0a9f2a"},
		{40, fullHash},
		{50, fullHash},
		{0, fullHash},
		{-1, fullHash},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.length)), func(t *testing.T) {
			result := ShortHash(fullHash, tt.length)
			assert.Equal(t, tt.expected, result)
		})
	}
}
