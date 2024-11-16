# Build stage
FROM --platform=$BUILDPLATFORM golang:1.23-bookworm AS builder

ARG TARGETARCH
ARG TARGETOS

WORKDIR /app

# Installation des dépendances système nécessaires pour la compilation
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    gcc \
    libc-dev \
    sqlite3 && \
    rm -rf /var/lib/apt/lists/*

# Pour générer le go.sum :
# docker run --rm -v $(pwd):/app -w /app golang:1.23-bookworm go mod tidy

# Copie seulement des fichiers de configuration des dépendances Go pour tirer profit du cache
COPY go.mod go.sum ./

# Téléchargement des dépendances seulement (caché si go.mod/go.sum n'ont pas changé)
RUN go mod tidy && go mod download

# Copie du reste du code source
COPY . .

# Build avec les optimisations
RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-w -s" -o /usr/local/bin/zockimate ./cmd/zockimate

# Final stage
FROM --platform=$TARGETPLATFORM debian:bookworm-slim

RUN mkdir -p /var/lib/zockimate

# Installation des dépendances système en une seule couche
RUN apt-get update && \
    apt-get install -y --no-install-recommends gnupg2 && \
    echo "deb http://deb.debian.org/debian bookworm contrib" >> /etc/apt/sources.list && \
    echo "deb http://deb.debian.org/debian bookworm-backports main contrib" >> /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        util-linux \
        zfsutils-linux && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
    
# Copie du binaire depuis le stage de build
COPY --from=builder /usr/local/bin/zockimate /usr/local/bin/zockimate

ENTRYPOINT ["/usr/local/bin/zockimate"]
