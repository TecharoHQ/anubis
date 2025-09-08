FROM docker.io/library/node AS build

WORKDIR /app
COPY . .

RUN npm ci && npm run build

FROM ghcr.io/xe/nginx-micro
COPY --from=build /app/build /www
COPY ./manifest/cfg/nginx/nginx.conf /conf
LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"