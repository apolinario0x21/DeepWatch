# --- Estágio de build ---
FROM golang:1.27-alpine AS build

WORKDIR /src

# Cache de dependências: baixa os módulos antes de copiar o código.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Binário estático (CGO desabilitado) para rodar em imagem mínima.
# -trimpath remove caminhos absolutos; -ldflags reduz o tamanho.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/deepwatch ./cmd/deepwatch

# --- Imagem final mínima ---
FROM gcr.io/distroless/static:nonroot

# Metadados OCI.
LABEL org.opencontainers.image.title="DeepWatch" \
      org.opencontainers.image.description="App Go instrumentada com métricas Prometheus" \
      org.opencontainers.image.source="https://github.com/apolinario0x21/DeepWatch"

WORKDIR /
COPY --from=build /out/deepwatch /deepwatch

# distroless:nonroot já roda como usuário não-root (uid 65532).
USER nonroot:nonroot

EXPOSE 8080

# Healthcheck usando o próprio binário (imagem não possui shell/curl).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/deepwatch", "-healthcheck"]

ENTRYPOINT ["/deepwatch"]
