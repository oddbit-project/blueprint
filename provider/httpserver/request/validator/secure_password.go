package validator

import (
	"github.com/go-playground/validator/v10"
	"strings"
)

// Sample custom validation rule for secure passwords
func ValidateSecurePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	// Password complexity requirements
	hasUpperCase := false
	hasLowerCase := false
	hasNumber := false
	hasSpecial := false

	if len(password) < 8 {
		return false
	}

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpperCase = true
		case 'a' <= char && char <= 'z':
			hasLowerCase = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune(`!@#$%^&*()-_=+[]{}|;:'",.<>/?`, char):
			hasSpecial = true
		}
	}

	// Require at least 3 of the 4 character types
	count := 0
	if hasUpperCase {
		count++
	}
	if hasLowerCase {
		count++
	}
	if hasNumber {
		count++
	}
	if hasSpecial {
		count++
	}

	return count >= 3
}
