"""
Custom exceptions for HMAC client library.
"""


class HMACClientError(Exception):
    """Base exception for HMAC client errors."""
    pass


class InputTooLargeError(HMACClientError):
    """Raised when input data exceeds size limits."""
    pass


class InvalidSignatureError(HMACClientError):
    """Raised when HMAC signature verification fails."""
    pass


class InvalidTimestampError(HMACClientError):
    """Raised when timestamp is outside allowed window."""
    pass


class ConfigurationError(HMACClientError):
    """Raised when client configuration is invalid."""
    pass


class HTTPError(HMACClientError):
    """Raised when HTTP request fails."""
    pass