#!/bin/bash

# Test script for mTLS demo

set -e

echo "Testing mTLS Demo"
echo "==================="

# Check if certificates exist
if [ ! -d "certs" ] || [ ! -f "certs/ca.crt" ]; then
    echo "Generating certificates..."
    ./generate-certs.sh
fi

echo ""
echo "Building server..."
cd server
go mod tidy
go build -o mtls-server main.go

echo ""
echo "Building client..."
cd ../client
go mod tidy  
go build -o mtls-client main.go

echo ""
echo "Starting mTLS server..."
cd ../server
./mtls-server &
SERVER_PID=$!

# Wait for server to start
sleep 3

echo ""
echo "Testing with mTLS client..."
cd ../client
./mtls-client

echo ""
echo "Cleaning up..."
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

echo ""
echo "mTLS Demo test completed!"