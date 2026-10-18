# syntax=docker/dockerfile:1.7

FROM oven/bun:1.4.0

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl wget git openssl unzip zip build-essential && rm -rf /var/lib/apt/lists/* && \
    curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    ln -sf /usr/local/go/bin/go /usr/bin/go && \
    ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt

ENV PATH="/usr/local/go/bin:${PATH}"

COPY Overlord-Server/package.json Overlord-Server/bun.lock* /app/Overlord-Server/

WORKDIR /app/Overlord-Server

RUN bun install --frozen-lockfile

WORKDIR /app

COPY Overlord-Server/ /app/Overlord-Server/
COPY Overlord-Client/ /app/Overlord-Client/
COPY Overlord-Client/ /app/Overlord-Client-Source/
COPY Overlord-Client/ /app/Overlord-Server/dist/Overlord-Client/
COPY scripts/ /app/scripts/

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV DATA_DIR=/app/data
ENV OVERLORD_ROOT=/app
ENV OVERLORD_PUBLIC_ROOT=/app/Overlord-Server/public

WORKDIR /app/Overlord-Server

RUN go version
RUN wget --version
RUN bun --version

RUN bun run build

RUN bun run minify

RUN mkdir -p /app/data && chown -R 1000:1000 /app/Overlord-Client* /app/data /home/bun 2>/dev/null || true

USER root

EXPOSE 8080

WORKDIR /app/Overlord-Server

CMD ["bun", "dist/index.js"]
