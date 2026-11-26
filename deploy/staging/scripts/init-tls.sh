#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
mkdir -p tls/postgres
if [[ -e tls/ca.crt || -e tls/postgres/server.key ]]; then
  echo "TLS files already exist; refusing to overwrite" >&2
  exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
openssl req -x509 -newkey rsa:3072 -nodes -days 365 -subj '/CN=Inspectly Staging DB CA' -keyout "$tmp/ca.key" -out tls/ca.crt
openssl req -new -newkey rsa:2048 -nodes -subj '/CN=postgres' -keyout tls/postgres/server.key -out "$tmp/server.csr"
printf 'subjectAltName=DNS:postgres\nextendedKeyUsage=serverAuth\n' > "$tmp/ext"
openssl x509 -req -in "$tmp/server.csr" -CA tls/ca.crt -CAkey "$tmp/ca.key" -CAcreateserial -days 365 -extfile "$tmp/ext" -out tls/postgres/server.crt
chmod 755 tls tls/postgres
chmod 644 tls/ca.crt tls/postgres/server.crt
sudo chown 70:70 tls/postgres/server.key
sudo chmod 600 tls/postgres/server.key
openssl verify -CAfile tls/ca.crt -verify_hostname postgres tls/postgres/server.crt
