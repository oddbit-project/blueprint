package pin

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateNumeric(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr error
	}{
		{"length 4", 4, nil},
		{"length 6", 6, nil},
		{"length 9", 9, nil},
		{"length 12", 12, nil},
		{"zero length", 0, ErrInvalidLength},
		{"negative length", -1, ErrInvalidLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateNumeric(tt.length)
			if err != tt.wantErr {
				t.Errorf("GenerateNumeric(%d) error = %v, wantErr %v", tt.length, err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}

			// Verify length (accounting for dashes)
			stripped := stripDashes(got)
			if len(stripped) != tt.length {
				t.Errorf("GenerateNumeric(%d) got length %d (stripped), want %d", tt.length, len(stripped), tt.length)
			}

			// Verify only contains digits
			if !regexp.MustCompile(`^[0-9-]+$`).MatchString(got) {
				t.Errorf("GenerateNumeric(%d) = %q, contains non-numeric characters", tt.length, got)
			}
		})
	}
}

func TestGenerateAlphanumeric(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr error
	}{
		{"length 4", 4, nil},
		{"length 6", 6, nil},
		{"length 9", 9, nil},
		{"length 12", 12, nil},
		{"zero length", 0, ErrInvalidLength},
		{"negative length", -1, ErrInvalidLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateAlphanumeric(tt.length)
			if err != tt.wantErr {
				t.Errorf("GenerateAlphanumeric(%d) error = %v, wantErr %v", tt.length, err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}

			// Verify length (accounting for dashes)
			stripped := stripDashes(got)
			if len(stripped) != tt.length {
				t.Errorf("GenerateAlphanumeric(%d) got length %d (stripped), want %d", tt.length, len(stripped), tt.length)
			}

			// Verify only contains uppercase alphanumeric and dashes
			if !regexp.MustCompile(`^[0-9A-Z-]+$`).MatchString(got) {
				t.Errorf("GenerateAlphanumeric(%d) = %q, contains invalid characters", tt.length, got)
			}
		})
	}
}

func TestFormatWithDashes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123", "123"},
		{"1234", "123-4"},
		{"123456", "123-456"},
		{"1234567", "123-456-7"},
		{"123456789", "123-456-789"},
		{"AB", "AB"},
		{"ABCDEF", "ABC-DEF"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatWithDashes(tt.input)
			if got != tt.want {
				t.Errorf("formatWithDashes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareNumeric(t *testing.T) {
	tests := []struct {
		name string
		pin1 string
		pin2 string
		want bool
	}{
		{"exact match", "123-456", "123-456", true},
		{"match without dashes", "123456", "123456", true},
		{"match mixed dashes", "123-456", "123456", true},
		{"match mixed dashes reversed", "123456", "123-456", true},
		{"different pins", "123-456", "654-321", false},
		{"different length", "123-456", "123-4567", false},
		{"empty strings", "", "", true},
		{"one empty", "123", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareNumeric(tt.pin1, tt.pin2)
			if got != tt.want {
				t.Errorf("CompareNumeric(%q, %q) = %v, want %v", tt.pin1, tt.pin2, got, tt.want)
			}
		})
	}
}

func TestCompareAlphanumeric(t *testing.T) {
	tests := []struct {
		name string
		pin1 string
		pin2 string
		want bool
	}{
		{"exact match uppercase", "ABC-123", "ABC-123", true},
		{"exact match lowercase", "abc-123", "abc-123", true},
		{"case insensitive", "ABC-123", "abc-123", true},
		{"case insensitive mixed", "AbC-123", "aBc-123", true},
		{"match without dashes", "ABC123", "abc123", true},
		{"match mixed dashes", "ABC-123", "abc123", true},
		{"match mixed dashes reversed", "abc123", "ABC-123", true},
		{"different pins", "ABC-123", "XYZ-789", false},
		{"different length", "ABC-123", "ABC-1234", false},
		{"empty strings", "", "", true},
		{"one empty", "ABC", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareAlphanumeric(tt.pin1, tt.pin2)
			if got != tt.want {
				t.Errorf("CompareAlphanumeric(%q, %q) = %v, want %v", tt.pin1, tt.pin2, got, tt.want)
			}
		})
	}
}

func TestUniqueness(t *testing.T) {
	// Generate multiple PINs and ensure they're unique (high probability)
	seen := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		pin, err := GenerateNumeric(9)
		if err != nil {
			t.Fatalf("GenerateNumeric failed: %v", err)
		}
		stripped := stripDashes(pin)
		if seen[stripped] {
			t.Errorf("Duplicate PIN generated: %s", pin)
		}
		seen[stripped] = true
	}

	seen = make(map[string]bool)
	for i := 0; i < iterations; i++ {
		pin, err := GenerateAlphanumeric(9)
		if err != nil {
			t.Fatalf("GenerateAlphanumeric failed: %v", err)
		}
		stripped := stripDashes(pin)
		if seen[stripped] {
			t.Errorf("Duplicate alphanumeric PIN generated: %s", pin)
		}
		seen[stripped] = true
	}
}

func TestCharacterDistribution(t *testing.T) {
	// Basic sanity check that we're using the full charset
	// Generate many PINs and check we see variety in characters
	charCount := make(map[rune]int)
	iterations := 500

	for i := 0; i < iterations; i++ {
		pin, _ := GenerateAlphanumeric(12)
		for _, c := range stripDashes(pin) {
			charCount[c]++
		}
	}

	// We should see all 36 characters (0-9, A-Z) with enough iterations
	expectedChars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, c := range expectedChars {
		if charCount[c] == 0 {
			t.Errorf("Character %c was never generated in %d iterations", c, iterations)
		}
	}
}

func TestStripDashes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123-456", "123456"},
		{"123-456-789", "123456789"},
		{"123", "123"},
		{"", ""},
		{"---", ""},
		{"1-2-3", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripDashes(tt.input)
			if got != tt.want {
				t.Errorf("stripDashes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGeneratedPinFormat(t *testing.T) {
	// Verify dash placement is correct
	tests := []struct {
		length        int
		expectedParts []int // length of each part
	}{
		{3, []int{3}},
		{4, []int{3, 1}},
		{6, []int{3, 3}},
		{7, []int{3, 3, 1}},
		{9, []int{3, 3, 3}},
		{10, []int{3, 3, 3, 1}},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.length)), func(t *testing.T) {
			pin, err := GenerateNumeric(tt.length)
			if err != nil {
				t.Fatalf("GenerateNumeric(%d) failed: %v", tt.length, err)
			}

			parts := strings.Split(pin, "-")
			if len(parts) != len(tt.expectedParts) {
				t.Errorf("GenerateNumeric(%d) = %q, got %d parts, want %d parts",
					tt.length, pin, len(parts), len(tt.expectedParts))
				return
			}

			for i, part := range parts {
				if len(part) != tt.expectedParts[i] {
					t.Errorf("GenerateNumeric(%d) = %q, part %d has length %d, want %d",
						tt.length, pin, i, len(part), tt.expectedParts[i])
				}
			}
		})
	}
}
