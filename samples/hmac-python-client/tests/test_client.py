"""
Unit tests for HMAC client library.
"""

import datetime
import hashlib
import hmac
import json
import uuid
from unittest.mock import Mock, patch

import pytest

from hmac_client import (
    HMACClient,
    InputTooLargeError,
    InvalidSignatureError,
    ConfigurationError,
    HTTPError
)
from hmac_client.constants import (
    HEADER_HMAC_HASH,
    HEADER_HMAC_TIMESTAMP,
    HEADER_HMAC_NONCE,
    MAX_INPUT_SIZE
)


class TestHMACClient:
    """Test HMAC client functionality."""
    
    @pytest.fixture
    def client(self):
        """Create test client."""
        return HMACClient("http://localhost:8080", "test-secret-key")
    
    @pytest.fixture
    def large_data(self):
        """Create data larger than default limit."""
        return b"x" * (MAX_INPUT_SIZE + 1)
    
    def test_init_default_config(self):
        """Test client initialization with default config."""
        client = HMACClient("http://localhost:8080", "secret")
        
        assert client.base_url == "http://localhost:8080"
        assert client.secret_key == "secret"
        assert client.config['key_interval'] == 300
        assert client.config['max_input_size'] == 33554432
        assert client.config['timeout'] == 30
    
    def test_init_custom_config(self):
        """Test client initialization with custom config."""
        client = HMACClient(
            "http://example.com/",
            "secret",
            key_interval=600,
            max_input_size=1024,
            timeout=60
        )
        
        assert client.base_url == "http://example.com"
        assert client.config['key_interval'] == 600
        assert client.config['max_input_size'] == 1024
        assert client.config['timeout'] == 60
    
    def test_init_invalid_config(self):
        """Test client initialization with invalid config."""
        with pytest.raises(ConfigurationError):
            HMACClient("http://localhost:8080", "")
        
        with pytest.raises(ConfigurationError):
            HMACClient("http://localhost:8080", "secret", key_interval=0)
        
        with pytest.raises(ConfigurationError):
            HMACClient("http://localhost:8080", "secret", max_input_size=-1)
    
    def test_sha256_sign(self, client):
        """Test simple SHA256 signing."""
        data = b"test data"
        signature = client.sha256_sign(data)
        
        # Verify it's a hex string
        assert isinstance(signature, str)
        assert len(signature) == 64  # SHA256 hex = 64 chars
        int(signature, 16)  # Should not raise
        
        # Verify signature is correct
        expected = hmac.new(
            b"test-secret-key",
            data,
            hashlib.sha256
        ).hexdigest()
        assert signature == expected
    
    def test_sha256_sign_large_data(self, client, large_data):
        """Test SHA256 signing with large data."""
        with pytest.raises(InputTooLargeError):
            client.sha256_sign(large_data)
    
    def test_sha256_verify_valid(self, client):
        """Test SHA256 verification with valid signature."""
        data = b"test data"
        signature = client.sha256_sign(data)
        
        assert client.sha256_verify(data, signature) is True
    
    def test_sha256_verify_invalid(self, client):
        """Test SHA256 verification with invalid signature."""
        data = b"test data"
        invalid_signature = "invalid"
        
        assert client.sha256_verify(data, invalid_signature) is False
    
    def test_sha256_verify_wrong_data(self, client):
        """Test SHA256 verification with wrong data."""
        data = b"test data"
        signature = client.sha256_sign(data)
        wrong_data = b"wrong data"
        
        assert client.sha256_verify(wrong_data, signature) is False
    
    def test_sha256_verify_large_data(self, client, large_data):
        """Test SHA256 verification with large data."""
        with pytest.raises(InputTooLargeError):
            client.sha256_verify(large_data, "signature")
    
    def test_sign256(self, client):
        """Test secure signing with timestamp and nonce."""
        data = b"test data"
        hash_value, timestamp, nonce = client.sign256(data)
        
        # Verify return types and formats
        assert isinstance(hash_value, str)
        assert isinstance(timestamp, str)
        assert isinstance(nonce, str)
        
        # Verify hash format
        assert len(hash_value) == 64
        int(hash_value, 16)  # Should not raise
        
        # Verify timestamp format (ISO format)
        datetime.datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
        
        # Verify nonce format (UUID)
        uuid.UUID(nonce)
        
        # Verify signature is correct
        message = f"{timestamp}:{nonce}:{data.decode('utf-8')}"
        expected = hmac.new(
            b"test-secret-key",
            message.encode('utf-8'),
            hashlib.sha256
        ).hexdigest()
        assert hash_value == expected
    
    def test_sign256_unique_nonces(self, client):
        """Test that sign256 generates unique nonces."""
        data = b"test data"
        
        _, _, nonce1 = client.sign256(data)
        _, _, nonce2 = client.sign256(data)
        
        assert nonce1 != nonce2
    
    def test_sign256_large_data(self, client, large_data):
        """Test sign256 with large data."""
        with pytest.raises(InputTooLargeError):
            client.sign256(large_data)
    
    def test_verify256_valid(self, client):
        """Test verify256 with valid signature."""
        data = b"test data"
        hash_value, timestamp, nonce = client.sign256(data)
        
        assert client.verify256(data, hash_value, timestamp, nonce) is True
    
    def test_verify256_invalid_signature(self, client):
        """Test verify256 with invalid signature."""
        data = b"test data"
        _, timestamp, nonce = client.sign256(data)
        invalid_hash = "invalid"
        
        assert client.verify256(data, invalid_hash, timestamp, nonce) is False
    
    def test_verify256_wrong_data(self, client):
        """Test verify256 with wrong data."""
        data = b"test data"
        hash_value, timestamp, nonce = client.sign256(data)
        wrong_data = b"wrong data"
        
        assert client.verify256(wrong_data, hash_value, timestamp, nonce) is False
    
    def test_verify256_expired_timestamp(self, client):
        """Test verify256 with expired timestamp."""
        data = b"test data"
        
        # Create old timestamp
        old_time = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=400)
        old_timestamp = old_time.isoformat()
        nonce = str(uuid.uuid4())
        
        # Generate signature with old timestamp
        message = f"{old_timestamp}:{nonce}:{data.decode('utf-8')}"
        hash_value = hmac.new(
            b"test-secret-key",
            message.encode('utf-8'),
            hashlib.sha256
        ).hexdigest()
        
        assert client.verify256(data, hash_value, old_timestamp, nonce) is False
    
    def test_verify256_future_timestamp(self, client):
        """Test verify256 with future timestamp within tolerance."""
        data = b"test data"
        
        # Create future timestamp within tolerance
        future_time = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=200)
        future_timestamp = future_time.isoformat()
        nonce = str(uuid.uuid4())
        
        # Generate signature with future timestamp
        message = f"{future_timestamp}:{nonce}:{data.decode('utf-8')}"
        hash_value = hmac.new(
            b"test-secret-key",
            message.encode('utf-8'),
            hashlib.sha256
        ).hexdigest()
        
        assert client.verify256(data, hash_value, future_timestamp, nonce) is True
    
    def test_verify256_large_data(self, client, large_data):
        """Test verify256 with large data."""
        with pytest.raises(InputTooLargeError):
            client.verify256(large_data, "hash", "timestamp", "nonce")
    
    def test_prepare_request_body_json(self, client):
        """Test request body preparation with JSON data."""
        json_data = {"key": "value", "number": 42}
        body = client._prepare_request_body(json_data=json_data)
        
        expected = json.dumps(json_data, separators=(',', ':')).encode('utf-8')
        assert body == expected
    
    def test_prepare_request_body_string(self, client):
        """Test request body preparation with string data."""
        data = "test string"
        body = client._prepare_request_body(data=data)
        
        assert body == b"test string"
    
    def test_prepare_request_body_bytes(self, client):
        """Test request body preparation with bytes data."""
        data = b"test bytes"
        body = client._prepare_request_body(data=data)
        
        assert body == b"test bytes"
    
    def test_prepare_request_body_empty(self, client):
        """Test request body preparation with no data."""
        body = client._prepare_request_body()
        
        assert body == b""
    
    @patch('hmac_client.client.requests.Session.request')
    def test_make_request_headers(self, mock_request, client):
        """Test that HTTP requests include proper HMAC headers."""
        mock_response = Mock()
        mock_request.return_value = mock_response
        
        client._make_request('GET', '/test')
        
        # Verify request was made
        mock_request.assert_called_once()
        args, kwargs = mock_request.call_args
        
        # Check headers
        headers = kwargs['headers']
        assert HEADER_HMAC_HASH in headers
        assert HEADER_HMAC_TIMESTAMP in headers
        assert HEADER_HMAC_NONCE in headers
        
        # Verify header formats
        assert len(headers[HEADER_HMAC_HASH]) == 64  # SHA256 hex
        datetime.datetime.fromisoformat(headers[HEADER_HMAC_TIMESTAMP].replace('Z', '+00:00'))
        uuid.UUID(headers[HEADER_HMAC_NONCE])
    
    @patch('hmac_client.client.requests.Session.request')
    def test_make_request_json(self, mock_request, client):
        """Test HTTP request with JSON data."""
        mock_response = Mock()
        mock_request.return_value = mock_response
        
        json_data = {"test": "data"}
        client._make_request('POST', '/test', json_data=json_data)
        
        args, kwargs = mock_request.call_args
        
        # Check content type
        assert kwargs['headers']['Content-Type'] == 'application/json'
        
        # Check body
        expected_body = json.dumps(json_data, separators=(',', ':')).encode('utf-8')
        assert kwargs['data'] == expected_body
    
    @patch('hmac_client.client.requests.Session.request')
    def test_http_methods(self, mock_request, client):
        """Test all HTTP method shortcuts."""
        mock_response = Mock()
        mock_request.return_value = mock_response
        
        # Test each method
        client.get('/test')
        client.post('/test', json={"data": "test"})
        client.put('/test', data="test")
        client.delete('/test')
        
        # Verify all calls were made
        assert mock_request.call_count == 4
        
        # Check methods
        calls = mock_request.call_args_list
        assert calls[0][0][0] == 'GET'
        assert calls[1][0][0] == 'POST'
        assert calls[2][0][0] == 'PUT'
        assert calls[3][0][0] == 'DELETE'
    
    def test_context_manager(self):
        """Test client as context manager."""
        with HMACClient("http://localhost:8080", "secret") as client:
            assert client.session is not None
        
        # Session should be closed after context exit
        # (Note: We can't easily test this without mocking)
    
    def test_verify_timestamp_valid(self, client):
        """Test timestamp verification with valid timestamp."""
        now = datetime.datetime.now(datetime.timezone.utc)
        timestamp = now.isoformat()
        
        assert client._verify_timestamp(timestamp) is True
    
    def test_verify_timestamp_invalid_format(self, client):
        """Test timestamp verification with invalid format."""
        assert client._verify_timestamp("invalid") is False
    
    def test_verify_timestamp_too_old(self, client):
        """Test timestamp verification with too old timestamp."""
        old_time = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=400)
        timestamp = old_time.isoformat()
        
        assert client._verify_timestamp(timestamp) is False
    
    def test_check_input_size_valid(self, client):
        """Test input size check with valid data."""
        data = b"small data"
        client._check_input_size(data)  # Should not raise
    
    def test_check_input_size_too_large(self, client, large_data):
        """Test input size check with too large data."""
        with pytest.raises(InputTooLargeError):
            client._check_input_size(large_data)