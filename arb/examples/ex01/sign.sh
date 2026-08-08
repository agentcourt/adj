#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname "$0")"

private_key="samantha_private.pem"
public_key="samantha_public.pem"
message="confession.txt"
signature="confession.sig"
signature_b64="confession.sig.b64"
decoded_signature="$(mktemp)"
trap 'rm -f "$decoded_signature"' EXIT

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$private_key"
openssl pkey -in "$private_key" -pubout -out "$public_key"
openssl dgst -sha256 -sign "$private_key" -out "$signature" "$message"
openssl dgst -sha256 -verify "$public_key" -signature "$signature" "$message"
base64 < "$signature" | tr -d '\n' > "$signature_b64"
base64 -d < "$signature_b64" > "$decoded_signature"
openssl dgst -sha256 -verify "$public_key" -signature "$decoded_signature" "$message"
