package hashing

// RehashFn is a function that returns a new hash for a password.
// It is returned by Verify when the hash parameters have changed
// and the password needs to be rehashed with updated security parameters.
type RehashFn = func() (string, error)

// PasswordHasher defines the interface for password hashing implementations.
// Implementations should use secure, modern hashing algorithms like Argon2,
// bcrypt, or scrypt.
type PasswordHasher interface {
	// Generate creates a secure hash from the given password.
	// The implementation should generate a random salt and include
	// all parameters needed for verification in the returned hash string.
	Generate(password string) (string, error)

	// Verify checks if the given password matches the hash.
	// Returns:
	//   - bool: true if the password matches the hash
	//   - RehashFn: non-nil if the hash needs updating with new parameters
	//   - error: any error that occurred during verification
	//
	// If the password is valid but the hash was created with outdated
	// parameters, the RehashFn should be called to generate a new hash.
	Verify(password, hash string) (bool, RehashFn, error)
}

type a2Hasher struct {
	cfg *Argon2Config
}

// NewArgon2Hasher creates a new PasswordHasher using the Argon2id algorithm.
// If cfg is nil, it uses the default configuration from NewArgon2IdConfig()
// with 64MB memory, 4 iterations, and parallelism based on CPU cores.
func NewArgon2Hasher(cfg *Argon2Config) (PasswordHasher, error) {
	if cfg == nil {
		cfg = NewArgon2IdConfig()
	}
	return &a2Hasher{
		cfg: cfg,
	}, nil
}

// Generate creates a secure Argon2id hash from the given password.
// The returned hash string contains all parameters needed for verification
// in the format: $argon2id$v=19$m=65536,t=4,p=8$salt$hash
func (h *a2Hasher) Generate(password string) (string, error) {
	return Argon2IdCreateHash(h.cfg, password)
}

// Verify checks if the password matches the given Argon2id hash.
// If the password is valid but was hashed with different parameters
// than the current configuration, it returns a RehashFn that can be
// used to generate a new hash with updated parameters.
func (h *a2Hasher) Verify(password, hash string) (bool, RehashFn, error) {
	valid, cfg, err := Argon2IdComparePassword(password, hash)
	if !valid || err != nil {
		return false, nil, err
	}

	// Check if the stored config differs from the hasher's current config
	if !h.needsRehash(cfg) {
		return valid, nil, nil
	}

	// return anonymous function to perform rehash
	fn := func() (string, error) {
		return Argon2IdCreateHash(h.cfg, password)
	}
	return true, fn, nil
}

// needsRehash checks if a hash was created with different parameters
// than the hasher's current configuration.
func (h *a2Hasher) needsRehash(storedCfg *Argon2Config) bool {
	return storedCfg.Memory != h.cfg.Memory ||
		storedCfg.Iterations != h.cfg.Iterations ||
		storedCfg.Parallelism != h.cfg.Parallelism ||
		storedCfg.SaltLength != h.cfg.SaltLength ||
		storedCfg.KeyLength != h.cfg.KeyLength
}
