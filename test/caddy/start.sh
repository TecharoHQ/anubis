#!/usr/bin/env bash

# If the transient local TLS certificate doesn't exist, mint a new one
if [ ! -f ../pki/caddy.local.cetacean.club/cert.pem ]; then
  # Subshell to contain the directory change
  (
    cd ../pki \
    && mkdir -p caddy.local.cetacean.club \
    && \
    # Try using https://github.com/FiloSottile/mkcert for better DevEx,
    # but fall back to using https://github.com/jsha/minica in case
    # you don't have that installed.
    (
      mkcert \
        --cert-file ./caddy.local.cetacean.club/cert.pem \
        --key-file ./caddy.local.cetacean.club/key.pem caddy.local.cetacean.club \
      || go tool minica -domains caddy.local.cetacean.club
    )
  )
fi

docker compose up --build