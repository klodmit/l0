# ===== builder =====
FROM golang:1.25-alpine AS builder
WORKDIR /src

# чтобы go мог подтянуть нужный toolchain при toolchain-директиве в go.mod
ENV GOTOOLCHAIN=auto

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${GIT_COMMIT}" \
    -o /out/l0 ./cmd/l0

# ===== runtime =====
FROM gcr.io/distroless/static-debian12 AS runtime
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/l0 /app/l0
ENV GIN_MODE=release
EXPOSE 8082
ENTRYPOINT ["/app/l0"]
