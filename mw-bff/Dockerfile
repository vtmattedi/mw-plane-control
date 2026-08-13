FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILT_AT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.Version=${VERSION} \
    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.Commit=${COMMIT} \
    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.BuiltAt=${BUILT_AT}" \
    -o /out/server \
    ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/data \
    && chown -R app:app /app

WORKDIR /app

COPY --from=build /out/server /app/server

USER app

EXPOSE 7200

ENTRYPOINT ["/app/server"]
