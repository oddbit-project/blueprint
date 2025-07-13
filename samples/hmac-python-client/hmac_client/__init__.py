"""
HMAC Client Library for Blueprint Framework

A Python client library that generates HMAC-signed requests compatible
with the Blueprint Go HMAC provider implementation.

Example usage:
    from hmac_client import HMACClient
    
    client = HMACClient("http://localhost:8080", "your-secret-key")
    response = client.get("/api/protected/profile")
"""

from .client import HMACClient
from .exceptions import (
    HMACClientError,
    InputTooLargeError,
    InvalidSignatureError,
    InvalidTimestampError,
    ConfigurationError,
    HTTPError
)
from .constants import (
    HEADER_HMAC_HASH,
    HEADER_HMAC_TIMESTAMP,
    HEADER_HMAC_NONCE,
    DEFAULT_CONFIG,
    MAX_INPUT_SIZE,
    DEFAULT_KEY_INTERVAL
)

__version__ = "1.0.0"
__author__ = "Blueprint Framework"
__all__ = [
    "HMACClient",
    "HMACClientError",
    "InputTooLargeError", 
    "InvalidSignatureError",
    "InvalidTimestampError",
    "ConfigurationError",
    "HTTPError",
    "HEADER_HMAC_HASH",
    "HEADER_HMAC_TIMESTAMP", 
    "HEADER_HMAC_NONCE",
    "DEFAULT_CONFIG",
    "MAX_INPUT_SIZE",
    "DEFAULT_KEY_INTERVAL"
]