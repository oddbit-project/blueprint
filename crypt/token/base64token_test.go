package token

import (
	"encoding/base64"
	"testing"
)

func TestGenerateSecureBase64Token(t *testing.T) {
	tests := []struct {
		name       string
		byteLength int
		wantErr    error
		wantLen    int
	}{
		{name: "zero bytes", byteLength: 0, wantErr: nil, wantLen: 0},
		{name: "16 bytes", byteLength: 16, wantErr: nil, wantLen: 16},
		{name: "32 bytes", byteLength: 32, wantErr: nil, wantLen: 32},
		{name: "negative", byteLength: -1, wantErr: ErrInvalidByteLength, wantLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateSecureBase64Token(tt.byteLength)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			decoded, err := base64.RawURLEncoding.DecodeString(token)
			if err != nil {
				t.Fatalf("generated token is not valid raw url base64: %v", err)
			}
			if len(decoded) != tt.wantLen {
				t.Fatalf("decoded token length = %d, want %d", len(decoded), tt.wantLen)
			}
		})
	}
}

func TestGenerateSecureBase64TokenUniqueness(t *testing.T) {
	t1, err := GenerateSecureBase64Token(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t2, err := GenerateSecureBase64Token(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if t1 == t2 {
		t.Fatalf("expected tokens to differ")
	}
}
