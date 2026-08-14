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
    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.Version=${VERSION} \
    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.Commit=${COMMIT} \
    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.BuiltAt=${BUILT_AT}" \
    -o /out/server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/create-superadmin \
    ./cmd/create-superadmin

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/create-application \
    ./cmd/create-application

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/rotate-signing-key \
    ./cmd/rotate-signing-key

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/recover-identity \
    ./cmd/recover-identity

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/assign-application \
    ./cmd/assign-application

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/backup-db \
    ./cmd/backup-db

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/verify-db \
    ./cmd/verify-db

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/maintenance \
    ./cmd/maintenance

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/config-check \
    ./cmd/config-check

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/data \
    && chown -R app:app /app

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /out/create-superadmin /app/create-superadmin
COPY --from=build /out/create-application /app/create-application
COPY --from=build /out/rotate-signing-key /app/rotate-signing-key
COPY --from=build /out/recover-identity /app/recover-identity
COPY --from=build /out/assign-application /app/assign-application

COPY --from=build /out/backup-db /app/backup-db
COPY --from=build /out/verify-db /app/verify-db
COPY --from=build /out/maintenance /app/maintenance
COPY --from=build /out/config-check /app/config-check

USER app

EXPOSE 7100

ENTRYPOINT ["/app/server"]
