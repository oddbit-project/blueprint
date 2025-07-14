"""
HMAC client library compatible with Blueprint Go HMAC provider.

This module provides HMAC-SHA256 signature generation and verification
compatible with the Blueprint framework's Go implementation.
"""

import datetime
import hashlib
import hmac
import json
import uuid
from typing import Dict, Optional, Tuple, Union, Any
from urllib.parse import urljoin

import requests

from .constants import (
    HEADER_HMAC_HASH,
    HEADER_HMAC_TIMESTAMP, 
    HEADER_HMAC_NONCE,
    DEFAULT_CONFIG,
    MAX_INPUT_SIZE
)
from .exceptions import (
    InputTooLargeError,
    InvalidSignatureError,
    InvalidTimestampError,
    ConfigurationError,
    HTTPError
)


class HMACClient:
    """
    HMAC client for making authenticated requests to Blueprint Go servers.
    
    Provides both simple HMAC (without nonce/timestamp) and secure HMAC
    (with nonce/timestamp) compatible with Blueprint's Go implementation.
    """
    
    def __init__(self, base_url: str, secret_key: str, **config):
        """
        Initialize HMAC client.
        
        Args:
            base_url: Base URL for HTTP requests
            secret_key: HMAC secret key (must match server)
            **config: Configuration options (key_interval, max_input_size, timeout)
        """
        self.base_url = base_url.rstrip('/')
        self.secret_key = secret_key
        
        # Merge default config with user overrides
        self.config = {**DEFAULT_CONFIG, **config}
        
        # Validate configuration
        self._validate_config()
        
        # Create HTTP session
        self.session = requests.Session()
        self.session.timeout = self.config['timeout']
    
    def _validate_config(self):
        """Validate client configuration."""
        if not self.secret_key:
            raise ConfigurationError("secret_key cannot be empty")
        
        if self.config['key_interval'] <= 0:
            raise ConfigurationError("key_interval must be positive")
            
        if self.config['max_input_size'] <= 0:
            raise ConfigurationError("max_input_size must be positive")
    
    def _check_input_size(self, data: bytes):
        """Check if input data exceeds size limit."""
        if len(data) > self.config['max_input_size']:
            raise InputTooLargeError(
                f"Input size {len(data)} exceeds limit {self.config['max_input_size']}"
            )
    
    def sha256_sign(self, data: bytes) -> str:
        """
        Generate simple HMAC-SHA256 signature (no nonce/timestamp).
        
        Compatible with Go HMACProvider.SHA256Sign().
        
        Args:
            data: Data to sign
            
        Returns:
            Hex-encoded HMAC signature
            
        Raises:
            InputTooLargeError: If data exceeds size limit
        """
        self._check_input_size(data)
        
        mac = hmac.new(
            self.secret_key.encode('utf-8'),
            data,
            hashlib.sha256
        )
        return mac.hexdigest()
    
    def sha256_verify(self, data: bytes, signature: str) -> bool:
        """
        Verify simple HMAC-SHA256 signature.
        
        Compatible with Go HMACProvider.SHA256Verify().
        
        Args:
            data: Original data
            signature: Hex-encoded signature to verify
            
        Returns:
            True if signature is valid
            
        Raises:
            InputTooLargeError: If data exceeds size limit
        """
        try:
            self._check_input_size(data)
            expected_signature = self.sha256_sign(data)
            
            # Use constant-time comparison to prevent timing attacks
            return hmac.compare_digest(expected_signature, signature)
        except InputTooLargeError:
            raise
        except Exception:
            return False
    
    def sign256(self, data: bytes) -> Tuple[str, str, str]:
        """
        Generate secure HMAC-SHA256 signature with timestamp and nonce.
        
        Compatible with Go HMACProvider.Sign256().
        Format: HMAC-SHA256(timestamp + ":" + nonce + ":" + data)
        
        Args:
            data: Data to sign
            
        Returns:
            Tuple of (hash, timestamp, nonce)
            
        Raises:
            InputTooLargeError: If data exceeds size limit
        """
        self._check_input_size(data)
        
        # Generate timestamp in RFC3339 format (matching Go time.RFC3339)
        timestamp = datetime.datetime.now(datetime.timezone.utc).isoformat()
        
        # Generate UUID v4 nonce (matching Go uuid.New().String())
        nonce = str(uuid.uuid4())
        
        # Build message: timestamp:nonce:data (matching Go implementation)
        message = f"{timestamp}:{nonce}:{data.decode('utf-8')}"
        
        # Generate HMAC signature
        mac = hmac.new(
            self.secret_key.encode('utf-8'),
            message.encode('utf-8'),
            hashlib.sha256
        )
        hash_value = mac.hexdigest()
        
        return hash_value, timestamp, nonce
    
    def verify256(self, data: bytes, hash_value: str, timestamp: str, nonce: str) -> bool:
        """
        Verify secure HMAC-SHA256 signature with timestamp and nonce.
        
        Compatible with Go HMACProvider.Verify256().
        
        Args:
            data: Original data
            hash_value: Hex-encoded signature
            timestamp: RFC3339 timestamp
            nonce: UUID nonce
            
        Returns:
            True if signature is valid and timestamp is within tolerance
            
        Raises:
            InputTooLargeError: If data exceeds size limit
        """
        try:
            self._check_input_size(data)
            
            # Validate timestamp first (before checking signature)
            if not self._verify_timestamp(timestamp):
                return False
            
            # Rebuild message and verify signature
            message = f"{timestamp}:{nonce}:{data.decode('utf-8')}"
            expected_mac = hmac.new(
                self.secret_key.encode('utf-8'),
                message.encode('utf-8'),
                hashlib.sha256
            )
            expected_hash = expected_mac.hexdigest()
            
            # Use constant-time comparison
            return hmac.compare_digest(expected_hash, hash_value)
        except InputTooLargeError:
            raise
        except Exception:
            return False
    
    def _verify_timestamp(self, timestamp: str) -> bool:
        """
        Verify timestamp is within allowed tolerance.
        
        Args:
            timestamp: RFC3339 timestamp string
            
        Returns:
            True if timestamp is valid and within tolerance
        """
        try:
            # Parse timestamp
            ts = datetime.datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
            now = datetime.datetime.now(datetime.timezone.utc)
            
            # Check if within tolerance window
            diff = abs((now - ts).total_seconds())
            return diff <= self.config['key_interval']
        except Exception:
            return False
    
    def _prepare_request_body(self, json_data=None, data=None) -> bytes:
        """Prepare request body for signing."""
        if json_data is not None:
            return json.dumps(json_data, separators=(',', ':')).encode('utf-8')
        elif data is not None:
            if isinstance(data, str):
                return data.encode('utf-8')
            elif isinstance(data, bytes):
                return data
            else:
                return str(data).encode('utf-8')
        else:
            return b''
    
    def _make_request(self, method: str, path: str, json_data=None, data=None, **kwargs) -> requests.Response:
        """
        Make authenticated HTTP request with HMAC headers.
        
        Args:
            method: HTTP method
            path: URL path (relative to base_url)
            json_data: JSON data to send
            data: Raw data to send
            **kwargs: Additional requests arguments
            
        Returns:
            requests.Response object
            
        Raises:
            HTTPError: If request fails
        """
        url = urljoin(self.base_url + '/', path.lstrip('/'))
        
        # Prepare request body
        body = self._prepare_request_body(json_data, data)
        
        # Generate HMAC signature
        hash_value, timestamp, nonce = self.sign256(body)
        
        # Add HMAC headers
        headers = kwargs.get('headers', {})
        headers.update({
            HEADER_HMAC_HASH: hash_value,
            HEADER_HMAC_TIMESTAMP: timestamp,
            HEADER_HMAC_NONCE: nonce
        })
        
        # Set content type for JSON
        if json_data is not None:
            headers['Content-Type'] = 'application/json'
        
        kwargs['headers'] = headers
        
        # Set request body
        if body:
            kwargs['data'] = body
        
        try:
            response = self.session.request(method, url, **kwargs)
            return response
        except requests.RequestException as e:
            raise HTTPError(f"HTTP request failed: {e}")
    
    def get(self, path: str, **kwargs) -> requests.Response:
        """Make authenticated GET request."""
        return self._make_request('GET', path, **kwargs)
    
    def post(self, path: str, json=None, data=None, **kwargs) -> requests.Response:
        """Make authenticated POST request."""
        return self._make_request('POST', path, json_data=json, data=data, **kwargs)
    
    def put(self, path: str, json=None, data=None, **kwargs) -> requests.Response:
        """Make authenticated PUT request."""
        return self._make_request('PUT', path, json_data=json, data=data, **kwargs)
    
    def delete(self, path: str, **kwargs) -> requests.Response:
        """Make authenticated DELETE request."""
        return self._make_request('DELETE', path, **kwargs)
    
    def close(self):
        """Close HTTP session."""
        if self.session:
            self.session.close()
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()