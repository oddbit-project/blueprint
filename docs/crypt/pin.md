# PIN Generation

The PIN generation package provides cryptographically secure PIN generation and comparison utilities.
It supports numeric and alphanumeric PINs with automatic dash formatting for readability.

## Features

- **Cryptographically secure** - Uses `crypto/rand` for secure random number generation
- **Numeric PINs** - Generate digit-only PINs (0-9)
- **Alphanumeric PINs** - Generate uppercase letter and digit PINs (A-Z, 0-9)
- **Auto-formatting** - Dashes inserted every 3 characters for readability
- **Constant-time comparison** - Timing attack protection using `crypto/subtle`
- **Case-insensitive matching** - Alphanumeric comparison ignores case

## Complete API Reference

### Generation Functions

#### GenerateNumeric

```go
func GenerateNumeric(length int) (string, error)
```

Generates a cryptographically secure numeric PIN of the specified length.

**Parameters:**
- `length`: The number of digits in the PIN (before formatting)

**Returns:**
- `string`: The generated PIN with dashes every 3 characters
- `error`: `ErrInvalidLength` if length is 0 or negative

**Example output:** `"123-456-789"` for length 9

#### GenerateAlphanumeric

```go
func GenerateAlphanumeric(length int) (string, error)
```

Generates a cryptographically secure alphanumeric PIN of the specified length.
The generated PIN contains uppercase letters (A-Z) and digits (0-9).

**Parameters:**
- `length`: The number of characters in the PIN (before formatting)

**Returns:**
- `string`: The generated PIN with dashes every 3 characters
- `error`: `ErrInvalidLength` if length is 0 or negative

**Example output:** `"AB3-XY7-Z9K"` for length 9

### Comparison Functions

#### CompareNumeric

```go
func CompareNumeric(pin1, pin2 string) bool
```

Performs a constant-time comparison of two numeric PINs.
Dashes are stripped before comparison, so `"123-456"` matches `"123456"`.

**Parameters:**
- `pin1`: First PIN to compare
- `pin2`: Second PIN to compare

**Returns:**
- `bool`: `true` if the PINs match, `false` otherwise

#### CompareAlphanumeric

```go
func CompareAlphanumeric(pin1, pin2 string) bool
```

Performs a constant-time, case-insensitive comparison of two alphanumeric PINs.
Dashes are stripped before comparison, so `"ABC-123"` matches `"abc123"`.

**Parameters:**
- `pin1`: First PIN to compare
- `pin2`: Second PIN to compare

**Returns:**
- `bool`: `true` if the PINs match (case-insensitive), `false` otherwise

### Error Constants

```go
var (
    ErrInvalidLength = errors.New("pin length must be greater than 0")
)
```

## Usage Examples

### Basic PIN Generation

```go
import (
    "github.com/oddbit-project/blueprint/crypt/pin"
    "log"
)

func basicExample() {
    // Generate a 6-digit numeric PIN
    numericPIN, err := pin.GenerateNumeric(6)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Numeric PIN: %s", numericPIN)
    // Output: Numeric PIN: 123-456

    // Generate an 8-character alphanumeric PIN
    alphaPIN, err := pin.GenerateAlphanumeric(8)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Alphanumeric PIN: %s", alphaPIN)
    // Output: Alphanumeric PIN: AB3-XY7-Z9
}
```

### PIN Verification

```go
func verificationExample() {
    // Generate a PIN
    storedPIN, _ := pin.GenerateNumeric(6)

    // User enters PIN (possibly without dashes)
    userInput := "123456"

    // Compare - dashes are automatically stripped
    if pin.CompareNumeric(storedPIN, userInput) {
        log.Println("PIN verified successfully")
    } else {
        log.Println("Invalid PIN")
    }
}
```

### Case-Insensitive Alphanumeric Comparison

```go
func caseInsensitiveExample() {
    // Generate alphanumeric PIN
    storedPIN, _ := pin.GenerateAlphanumeric(6)
    // storedPIN might be: "ABC-123"

    // User enters lowercase
    userInput := "abc123"

    // Comparison is case-insensitive
    if pin.CompareAlphanumeric(storedPIN, userInput) {
        log.Println("PIN verified successfully")
    }
}
```

### One-Time PIN (OTP) Use Case

```go
func otpExample() {
    // Generate a 6-digit OTP for email verification
    otp, err := pin.GenerateNumeric(6)
    if err != nil {
        log.Fatal(err)
    }

    // Send to user (formatted for readability)
    sendEmail(user.Email, fmt.Sprintf("Your verification code is: %s", otp))

    // Store for later verification
    storeOTP(user.ID, otp, time.Now().Add(15*time.Minute))
}

func verifyOTP(userID string, enteredOTP string) bool {
    storedOTP, expiry := getStoredOTP(userID)

    if time.Now().After(expiry) {
        return false // OTP expired
    }

    // Constant-time comparison prevents timing attacks
    return pin.CompareNumeric(storedOTP, enteredOTP)
}
```

### Account Recovery Code Generation

```go
func recoveryCodeExample() {
    // Generate 8 recovery codes for account recovery
    codes := make([]string, 8)
    for i := range codes {
        code, err := pin.GenerateAlphanumeric(9)
        if err != nil {
            log.Fatal(err)
        }
        codes[i] = code
    }

    // Display to user: ABC-DEF-GH1, XYZ-123-AB4, etc.
    for i, code := range codes {
        log.Printf("Recovery code %d: %s", i+1, code)
    }

    // Store hashed versions for security
    storeRecoveryCodes(user.ID, codes)
}
```

### Two-Factor Authentication Setup

```go
func twoFactorSetupExample() {
    // Generate backup codes for 2FA
    backupCodes := make([]string, 10)
    for i := range backupCodes {
        code, _ := pin.GenerateAlphanumeric(8)
        backupCodes[i] = code
    }

    // Present to user once
    log.Println("Save these backup codes in a safe place:")
    for _, code := range backupCodes {
        log.Printf("  %s", code)
    }
}
```

## Security Considerations

### Cryptographic Security

The package uses `crypto/rand` for random number generation, which provides cryptographically secure randomness suitable for security-sensitive applications.

### Timing Attack Protection

Both comparison functions use `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. This ensures that comparing PINs takes the same amount of time regardless of how many characters match.

```go
// Safe - uses constant-time comparison
if pin.CompareNumeric(storedPIN, userInput) {
    // Valid
}

// UNSAFE - do not use direct string comparison
if storedPIN == userInput {
    // Vulnerable to timing attacks
}
```

### PIN Length Recommendations

| Use Case | Recommended Length | Type |
|----------|-------------------|------|
| SMS verification | 6 digits | Numeric |
| Email verification | 6-8 digits | Numeric |
| Recovery codes | 8-12 characters | Alphanumeric |
| Temporary passwords | 12+ characters | Alphanumeric |

### Entropy Calculation

| Length | Numeric Entropy | Alphanumeric Entropy |
|--------|-----------------|---------------------|
| 4 | ~13 bits | ~21 bits |
| 6 | ~20 bits | ~31 bits |
| 8 | ~27 bits | ~41 bits |
| 10 | ~33 bits | ~52 bits |
| 12 | ~40 bits | ~62 bits |
