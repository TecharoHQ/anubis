FROM node AS build

WORKDIR /app
COPY . .

RUN npm ci && npm run build

FROM nginx:alpine
COPY --from=build /app/build /usr/share/nginx/html
LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"