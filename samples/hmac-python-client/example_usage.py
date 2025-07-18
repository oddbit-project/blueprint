#!/usr/bin/env python3
"""
Basic usage examples for HMAC Python client library.

This script demonstrates how to use the HMAC client library to make
authenticated requests to a Blueprint Go server.
"""

import json
import sys
import time
from hmac_client import HMACClient, HMACClientError


def main():
    """Run basic usage examples."""
    
    # Server configuration
    key_id = "client1"
    server_url = "http://localhost:8080"
    secret_key = "python-client-demo-secret"
    
    print("=== HMAC Python Client Basic Usage Examples ===\n")
    
    # Create HMAC client
    print("1. Creating HMAC client...")
    client = HMACClient(server_url, key_id, secret_key)
    print(f"   Client created for: {server_url}")
    print(f"   Key id: {key_id}\n")
    print(f"   Secret key: {secret_key[:8]}...\n")
    
    try:
        # Example 1: Test public endpoint (no authentication)
        print("2. Testing public endpoint (no authentication required)...")
        response = client.session.get(f"{server_url}/api/public/health")
        if response.status_code == 200:
            health_data = response.json()
            print(f"   ✓ Health check successful: {health_data['status']}")
            print(f"   Server: {health_data.get('service', 'Unknown')}")
        else:
            print(f"   ✗ Health check failed: {response.status_code}")
        print()
        
        # Example 2: Simple HMAC signing (no nonce/timestamp)
        print("3. Testing simple HMAC signing...")
        test_data = b"Hello, HMAC world!"
        signature = client.sha256_sign(test_data)
        print(f"   Data: {test_data}")
        print(f"   Signature: {signature}")
        
        # Verify the signature
        is_valid = client.sha256_verify(test_data, signature)
        print(f"   Verification: {'✓ Valid' if is_valid else '✗ Invalid'}")
        print()
        
        # Example 3: Secure HMAC signing (with nonce/timestamp)
        print("4. Testing secure HMAC signing...")
        hash_value, timestamp, nonce = client.sign256(test_data)
        print(f"   Data: {test_data}")
        print(f"   Hash: {hash_value}")
        print(f"   Timestamp: {timestamp}")
        print(f"   Nonce: {nonce}")
        
        # Verify the secure signature
        is_valid = client.verify256(test_data, hash_value, timestamp, nonce)
        print(f"   Verification: {'✓ Valid' if is_valid else '✗ Invalid'}")
        print()
        
        # Example 4: Authenticated GET request
        print("5. Testing authenticated GET request...")
        try:
            response = client.get("/api/protected/profile")
            if response.status_code == 200:
                profile_data = response.json()
                print(f"   ✓ GET request successful")
                print(f"   User: {profile_data['username']} ({profile_data['email']})")
                print(f"   Message: {profile_data.get('message', 'No message')}")
            else:
                print(f"   ✗ GET request failed: {response.status_code}")
                print(f"   Response: {response.text}")
        except Exception as e:
            print(f"   ✗ GET request error: {e}")
        print()
        
        # Example 5: Authenticated POST request with JSON
        print("6. Testing authenticated POST request with JSON...")
        post_data = {
            "message": "Hello from Python HMAC client!",
            "type": "example",
            "timestamp": time.time()
        }
        
        try:
            response = client.post("/api/protected/data", json=post_data)
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ POST request successful")
                print(f"   ID: {result['id']}")
                print(f"   Server: {result.get('server', 'Unknown')}")
                print(f"   Processed: {result.get('processed', 'Unknown')}")
            else:
                print(f"   ✗ POST request failed: {response.status_code}")
                print(f"   Response: {response.text}")
        except Exception as e:
            print(f"   ✗ POST request error: {e}")
        print()
        
        # Example 6: Authenticated PUT request
        print("7. Testing authenticated PUT request...")
        settings_data = {
            "theme": "dark",
            "language": "python", 
            "notifications": True,
            "auto_save": False
        }
        
        try:
            response = client.put("/api/protected/settings", json=settings_data)
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ PUT request successful")
                print(f"   Message: {result['message']}")
                print(f"   Updated: {result.get('updated_at', 'Unknown')}")
            else:
                print(f"   ✗ PUT request failed: {response.status_code}")
        except Exception as e:
            print(f"   ✗ PUT request error: {e}")
        print()
        
        # Example 7: Authenticated DELETE request
        print("8. Testing authenticated DELETE request...")
        resource_id = "example-resource-123"
        
        try:
            response = client.delete(f"/api/protected/resource/{resource_id}")
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ DELETE request successful")
                print(f"   Message: {result['message']}")
                print(f"   Resource ID: {result.get('resource_id', 'Unknown')}")
            else:
                print(f"   ✗ DELETE request failed: {response.status_code}")
        except Exception as e:
            print(f"   ✗ DELETE request error: {e}")
        print()
        
        # Example 8: Test endpoints
        print("9. Testing specialized test endpoints...")
        
        # Simple test
        try:
            response = client.get("/api/test/simple")
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ Simple test: {result['message']}")
            else:
                print(f"   ✗ Simple test failed: {response.status_code}")
        except Exception as e:
            print(f"   ✗ Simple test error: {e}")
        
        # JSON test
        try:
            test_payload = {"python_client": True, "test_number": 42}
            response = client.post("/api/test/json", json=test_payload)
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ JSON test: {result['message']}")
                print(f"   Received back: {result['received']}")
            else:
                print(f"   ✗ JSON test failed: {response.status_code}")
        except Exception as e:
            print(f"   ✗ JSON test error: {e}")
        
        # Large data test
        try:
            large_data = "x" * 5000  # 5KB test data
            response = client.post("/api/test/large", data=large_data)
            if response.status_code == 200:
                result = response.json()
                print(f"   ✓ Large data test: {result['message']}")
                print(f"   Data size: {result['size']} bytes")
            else:
                print(f"   ✗ Large data test failed: {response.status_code}")
        except Exception as e:
            print(f"   ✗ Large data test error: {e}")
        print()
        
        # Example 9: Error handling demonstration
        print("10. Demonstrating error handling...")
        
        # Try with wrong secret
        print("    Testing with wrong secret key...")
        wrong_client = HMACClient(server_url, key_id, "wrong-secret-key")
        try:
            response = wrong_client.get("/api/protected/profile")
            if response.status_code == 401:
                print("    ✓ Correctly rejected wrong secret (401 Unauthorized)")
            else:
                print(f"    ✗ Unexpected response: {response.status_code}")
        except Exception as e:
            print(f"    ✗ Wrong secret test error: {e}")
        
        # Try with invalid endpoint
        print("    Testing with invalid endpoint...")
        try:
            response = client.get("/api/nonexistent")
            if response.status_code == 404:
                print("    ✓ Correctly handled invalid endpoint (404 Not Found)")
            else:
                print(f"    ✓ Invalid endpoint response: {response.status_code}")
        except Exception as e:
            print(f"    ✗ Invalid endpoint test error: {e}")
        print()
        
        print("=== All Examples Completed Successfully! ===")
        
    except HMACClientError as e:
        print(f"HMAC Client Error: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"Unexpected error: {e}")
        sys.exit(1)
    finally:
        # Clean up
        client.close()


def demonstrate_context_manager():
    """Demonstrate using the client as a context manager."""
    
    print("\n=== Context Manager Usage Example ===")
    
    server_url = "http://localhost:8080"
    secret_key = "python-client-demo-secret"
    
    # Using context manager (recommended)
    try:
        with HMACClient(server_url, key_id, secret_key) as client:
            response = client.get("/api/test/simple")
            if response.status_code == 200:
                result = response.json()
                print(f"✓ Context manager test: {result['message']}")
            else:
                print(f"✗ Context manager test failed: {response.status_code}")
        # Client is automatically closed when exiting the context
        print("✓ Client automatically closed")
        
    except Exception as e:
        print(f"Context manager error: {e}")


def demonstrate_configuration():
    """Demonstrate client configuration options."""
    
    print("\n=== Configuration Options Example ===")
    
    # Create client with custom configuration
    client = HMACClient(
        "http://localhost:8080",
        "client1",
        "python-client-demo-secret",
        key_interval=600,        # 10 minutes tolerance
        max_input_size=1024000,  # 1MB limit
        timeout=60               # 60 second HTTP timeout
    )
    
    print(f"✓ Client configured with:")
    print(f"  - Key interval: {client.config['key_interval']} seconds")
    print(f"  - Max input size: {client.config['max_input_size']} bytes")
    print(f"  - HTTP timeout: {client.config['timeout']} seconds")
    
    client.close()


if __name__ == "__main__":
    # Check if server is running
    try:
        import requests
        response = requests.get("http://localhost:8080/api/public/health", timeout=5)
        if response.status_code != 200:
            print("Go server not running. Please start the server first:")
            print("> cd server && go run main.go")
            sys.exit(1)
    except Exception:
        print("Go server not accessible. Please start the server first:")
        print(">  cd server && go run main.go")
        sys.exit(1)
    
    # Run examples
    main()
    demonstrate_context_manager()
    demonstrate_configuration()
    
    print("\nAll examples completed! Check the server logs to see the authentication in action.")