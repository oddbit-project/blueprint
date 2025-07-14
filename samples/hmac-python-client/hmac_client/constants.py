"""
Constants for HMAC client library.
Compatible with Blueprint Go HMAC provider configuration.
"""

# HTTP Headers (matching provider/httpserver/auth/hmac.go:12-15)
HEADER_HMAC_HASH = "X-HMAC-Hash"
HEADER_HMAC_TIMESTAMP = "X-HMAC-Timestamp"
HEADER_HMAC_NONCE = "X-HMAC-Nonce"

# Default configuration values (matching provider/hmacprovider/container.go:16-19)
DEFAULT_CONFIG = {
    'key_interval': 300,        # 5 minutes in seconds (DefaultKeyInterval)
    'max_input_size': 33554432, # 32MB in bytes (MaxInputSize)
    'timeout': 30,              # HTTP timeout in seconds
}

# Other constants
MAX_INPUT_SIZE = 32 * 1024 * 1024  # 32MB
DEFAULT_KEY_INTERVAL = 5 * 60       # 5 minutes in seconds