#!/bin/bash

# Generate certificates for mTLS demo
# This script creates a CA, server certificate, and client certificate

set -e

CERT_DIR="certs"
mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

echo "Generating mTLS certificates..."

# 1. Create CA private key
echo "Creating CA private key..."
openssl genrsa -out ca.key 4096

# 2. Create CA certificate
echo "Creating CA certificate..."
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Security/CN=Blueprint Demo CA"

# 3. Create server private key
echo "Creating server private key..."
openssl genrsa -out server.key 4096

# 4. Create server certificate signing request
echo "Creating server certificate signing request..."
openssl req -new -key server.key -out server.csr \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Server/CN=localhost"

# 5. Create server certificate extensions file
cat > server.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = api.example.com
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# 6. Sign server certificate with CA
echo "Signing server certificate..."
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -extfile server.ext

# 7. Create client private key
echo "Creating client private key..."
openssl genrsa -out client.key 4096

# 8. Create client certificate signing request
echo "Creating client certificate signing request..."
openssl req -new -key client.key -out client.csr \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Client/CN=demo-client.example.com"

# 9. Create client certificate extensions file
cat > client.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = demo-client.example.com
DNS.2 = client.blueprint.demo
EOF

# 10. Sign client certificate with CA
echo "Signing client certificate..."
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out client.crt -extfile client.ext

# 11. Cleanup
rm -f *.csr *.ext ca.srl

echo "Certificate generation complete!"
echo ""
echo "Generated files:"
echo "  ca.crt       - Certificate Authority certificate"
echo "  ca.key       - Certificate Authority private key"
echo "  server.crt   - Server certificate"
echo "  server.key   - Server private key"
echo "  client.crt   - Client certificate" 
echo "  client.key   - Client private key"
echo ""
echo "Certificate details:"
echo "CA Certificate:"
openssl x509 -in ca.crt -text -noout | grep -E "(Subject:|Not Before|Not After)"
echo ""
echo "Server Certificate:"
openssl x509 -in server.crt -text -noout | grep -E "(Subject:|Not Before|Not After|DNS:|IP Address:)"
echo ""
echo "Client Certificate:"
openssl x509 -in client.crt -text -noout | grep -E "(Subject:|Not Before|Not After|DNS:)"