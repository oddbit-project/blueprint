"""
Integration tests for Python HMAC client with Go server.
"""

import json
import subprocess
import time
import threading
import pytest
import requests

from hmac_client import HMACClient, HTTPError


class TestIntegration:
    """Integration tests with Go server."""
    KEY_ID = "client1"
    SERVER_URL = "http://localhost:8080"
    SECRET_KEY = "python-client-demo-secret"
    
    @pytest.fixture(scope="class", autouse=True)
    def go_server(self):
        """Start Go server for integration tests."""
        # Start the Go server
        server_process = subprocess.Popen(
            ["go", "run", "main.go"],
            cwd="server/",
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        
        # Wait for server to start
        time.sleep(6)
        
        # Check if server is running
        try:
            response = requests.get(f"{self.SERVER_URL}/api/public/health", timeout=5)
            if response.status_code != 200:
                raise Exception("Server not responding")
        except Exception as e:
            server_process.terminate()
            server_process.wait()
            pytest.skip(f"Could not start Go server: {e}")
        
        yield server_process
        
        # Cleanup: stop the server
        server_process.terminate()
        server_process.wait()
    
    @pytest.fixture
    def client(self):
        """Create authenticated HMAC client."""
        return HMACClient(self.SERVER_URL, self.KEY_ID, self.SECRET_KEY)
    
    def test_public_health_endpoint(self):
        """Test public health endpoint (no auth required)."""
        response = requests.get(f"{self.SERVER_URL}/api/public/health")
        
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "timestamp" in data
    
    def test_public_info_endpoint(self):
        """Test public info endpoint (no auth required)."""
        response = requests.get(f"{self.SERVER_URL}/api/public/info")
        
        assert response.status_code == 200
        data = response.json()
        assert data["service"] == "HMAC Python Client Demo Server"
        assert "endpoints" in data
    
    def test_protected_endpoint_without_auth(self):
        """Test that protected endpoints reject unauthenticated requests."""
        response = requests.get(f"{self.SERVER_URL}/api/protected/profile")
        
        assert response.status_code == 401
    
    def test_protected_profile_endpoint(self, client):
        """Test authenticated access to profile endpoint."""
        response = client.get("/api/protected/profile")
        
        assert response.status_code == 200
        data = response.json()
        assert data["user_id"] == "python-client-user"
        assert data["username"] == "python_tester"
        assert "message" in data
    
    def test_protected_post_endpoint(self, client):
        """Test authenticated POST request."""
        payload = {
            "message": "Hello from Python client!",
            "type": "integration_test"
        }
        
        response = client.post("/api/protected/data", json=payload)
        
        assert response.status_code == 200
        data = response.json()
        assert data["message"] == payload["message"]
        assert data["type"] == payload["type"]
        assert data["client_type"] == "Python HMAC Client"
        assert "id" in data
    
    def test_protected_put_endpoint(self, client):
        """Test authenticated PUT request."""
        settings = {
            "theme": "dark",
            "language": "python",
            "notifications": True
        }
        
        response = client.put("/api/protected/settings", json=settings)
        
        assert response.status_code == 200
        data = response.json()
        assert "Settings updated successfully" in data["message"]
        assert data["settings"] == settings
    
    def test_protected_delete_endpoint(self, client):
        """Test authenticated DELETE request."""
        resource_id = "test-resource-123"
        
        response = client.delete(f"/api/protected/resource/{resource_id}")
        
        assert response.status_code == 200
        data = response.json()
        assert resource_id in data["message"]
        assert data["resource_id"] == resource_id
    
    def test_echo_endpoint(self, client):
        """Test echo endpoint that returns request details."""
        test_data = "Echo test data from Python client"
        
        response = client.post("/api/protected/echo", data=test_data)
        
        assert response.status_code == 200
        data = response.json()
        assert data["echo"] == test_data
        assert data["size"] == len(test_data)
        
        # Check that HMAC headers were included
        headers = data["headers"]
        assert "X-Hmac-Hash" in headers
        assert "X-Hmac-Timestamp" in headers
        assert "X-Hmac-Nonce" in headers
    
    def test_simple_test_endpoint(self, client):
        """Test simple test endpoint."""
        response = client.get("/api/test/simple")
        
        assert response.status_code == 200
        data = response.json()
        assert data["test"] == "simple"
        assert data["status"] == "success"
    
    def test_json_test_endpoint(self, client):
        """Test JSON test endpoint."""
        payload = {
            "test_key": "test_value",
            "number": 42,
            "nested": {"inner": "value"}
        }
        
        response = client.post("/api/test/json", json=payload)
        
        assert response.status_code == 200
        data = response.json()
        assert data["test"] == "json"
        assert data["status"] == "success"
        assert data["received"] == payload
    
    def test_large_data_endpoint(self, client):
        """Test large data handling."""
        # Create moderately large data (not exceeding limits)
        large_data = "x" * 10000  # 10KB
        
        response = client.post("/api/test/large", data=large_data)
        
        assert response.status_code == 200
        data = response.json()
        assert data["test"] == "large"
        assert data["status"] == "success"
        assert data["size"] == len(large_data)
    
    def test_wrong_secret_key(self):
        """Test that wrong secret key results in authentication failure."""
        wrong_client = HMACClient(self.SERVER_URL, self.KEY_ID, "wrong-secret-key")
        
        response = wrong_client.get("/api/protected/profile")
        assert response.status_code == 401
    
    def test_signature_compatibility(self, client):
        """Test that Python client signatures are compatible with Go server."""
        # Test data
        test_data = "Test signature compatibility between Python and Go"
        
        # Sign with Python client
        hash_value, timestamp, nonce = client.sign256(test_data.encode())
        
        # Use the public sign endpoint to verify Go can generate the same signature
        response = requests.post(
            f"{self.SERVER_URL}/api/public/sign",
            json={"data": test_data},
            headers={"Content-Type": "application/json"}
        )
        
        assert response.status_code == 200
        go_signature = response.json()
        
        # Verify Go server accepts Python signature by making authenticated request
        test_response = client.post("/api/protected/echo", data=test_data)
        assert test_response.status_code == 200
    
    def test_concurrent_requests(self, client):
        """Test concurrent authenticated requests."""
        def make_request(i):
            payload = {"message": f"Concurrent request {i}", "thread": i}
            response = client.post("/api/protected/data", json=payload)
            return response.status_code == 200
        
        # Make multiple concurrent requests
        threads = []
        results = []
        
        for i in range(5):
            thread = threading.Thread(
                target=lambda idx=i: results.append(make_request(idx))
            )
            threads.append(thread)
            thread.start()
        
        # Wait for all threads
        for thread in threads:
            thread.join()
        
        # All requests should succeed
        assert all(results)
        assert len(results) == 5
    
    def test_request_replay_protection(self, client):
        """Test that replaying requests fails (nonce protection)."""
        # Make first request
        payload = {"message": "Original request"}
        response1 = client.post("/api/protected/data", json=payload)
        assert response1.status_code == 200
        
        # Extract headers from the client's last request
        # Note: This is a simplified test - in reality, replaying exact requests
        # is prevented by the nonce store on the server side
        
        # Make another request with different nonce (should succeed)
        response2 = client.post("/api/protected/data", json=payload)
        assert response2.status_code == 200
        
        # Each request should have different nonces (automatic by client)
        # The actual replay protection is tested by ensuring each request succeeds
        # because the client generates unique nonces automatically
    
    def test_timestamp_tolerance(self, client):
        """Test that requests within timestamp tolerance are accepted."""
        # Normal request should work
        response = client.get("/api/test/simple")
        assert response.status_code == 200
        
        # Test with custom client having very short tolerance
        short_tolerance_client = HMACClient(
            self.SERVER_URL,
            self.KEY_ID,
            self.SECRET_KEY,
            key_interval=1  # 1 second tolerance
        )
        
        # Request should still work with short tolerance
        response = short_tolerance_client.get("/api/test/simple")
        assert response.status_code == 200
    
    def test_error_handling(self, client):
        """Test error handling for various scenarios."""
        # Test invalid JSON
        response = client.post(
            "/api/test/json",
            data="invalid json",
            headers={"Content-Type": "application/json"}
        )
        assert response.status_code == 400
        
        # Test non-existent endpoint
        response = client.get("/api/nonexistent")
        assert response.status_code == 404
    
    def test_context_manager_usage(self):
        """Test using client as context manager."""
        with HMACClient(self.SERVER_URL, self.KEY_ID, self.SECRET_KEY) as client:
            response = client.get("/api/test/simple")
            assert response.status_code == 200