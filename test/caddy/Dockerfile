# FROM caddy:2.10.0-builder AS builder

# RUN xcaddy build \
#   --with github.com/lolPants/caddy-requestid

FROM caddy:2.10.0 AS run

# COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile