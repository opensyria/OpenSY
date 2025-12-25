#!/bin/bash
# Generate self-signed TLS certificates for CoopMine
# Usage: ./gen_certs.sh [output_dir]

set -e

OUTPUT_DIR="${1:-./certs}"
mkdir -p "$OUTPUT_DIR"

echo "Generating TLS certificates in $OUTPUT_DIR"

# Generate CA key and certificate
openssl genrsa -out "$OUTPUT_DIR/ca.key" 4096
openssl req -new -x509 -days 3650 -key "$OUTPUT_DIR/ca.key" \
    -out "$OUTPUT_DIR/ca.crt" \
    -subj "/C=SY/ST=Syria/L=Damascus/O=OpenSY/OU=CoopMine/CN=CoopMine CA"

# Generate server key and certificate signing request
openssl genrsa -out "$OUTPUT_DIR/server.key" 2048
openssl req -new -key "$OUTPUT_DIR/server.key" \
    -out "$OUTPUT_DIR/server.csr" \
    -subj "/C=SY/ST=Syria/L=Damascus/O=OpenSY/OU=CoopMine/CN=coordinator"

# Create extensions file for SAN
cat > "$OUTPUT_DIR/server.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = coordinator
DNS.3 = *.coopmine.local
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Sign server certificate with CA
openssl x509 -req -in "$OUTPUT_DIR/server.csr" \
    -CA "$OUTPUT_DIR/ca.crt" -CAkey "$OUTPUT_DIR/ca.key" \
    -CAcreateserial -out "$OUTPUT_DIR/server.crt" \
    -days 365 -extfile "$OUTPUT_DIR/server.ext"

# Generate client key and certificate (for mutual TLS)
openssl genrsa -out "$OUTPUT_DIR/client.key" 2048
openssl req -new -key "$OUTPUT_DIR/client.key" \
    -out "$OUTPUT_DIR/client.csr" \
    -subj "/C=SY/ST=Syria/L=Damascus/O=OpenSY/OU=CoopMine/CN=worker"

openssl x509 -req -in "$OUTPUT_DIR/client.csr" \
    -CA "$OUTPUT_DIR/ca.crt" -CAkey "$OUTPUT_DIR/ca.key" \
    -CAcreateserial -out "$OUTPUT_DIR/client.crt" \
    -days 365

# Clean up CSR files
rm -f "$OUTPUT_DIR"/*.csr "$OUTPUT_DIR"/*.ext "$OUTPUT_DIR"/*.srl

echo ""
echo "Generated certificates:"
ls -la "$OUTPUT_DIR"
echo ""
echo "Usage:"
echo "  Coordinator: --tls-cert=$OUTPUT_DIR/server.crt --tls-key=$OUTPUT_DIR/server.key"
echo "  Worker:      --tls --tls-ca=$OUTPUT_DIR/ca.crt"
