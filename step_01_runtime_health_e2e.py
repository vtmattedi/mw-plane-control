#!/usr/bin/env python3
from __future__ import annotations

import argparse
import datetime as dt
import os
import shutil
import subprocess
import sys
from pathlib import Path

FILES = {'.dockerignore': '.git\n.gitignore\n.backup\n**/.backup\n**/node_modules\n**/dist\n**/data\n**/*.db\n**/*.db-shm\n**/*.db-wal\n', '.env.example': 'APP_ENV=development\nGATEWAY_HOST_PORT=3050\n', 'docker-compose.yml': 'services:\n  gateway:\n    build:\n      context: .\n      dockerfile: infra/gateway/Dockerfile\n    ports:\n      - "127.0.0.1:${GATEWAY_HOST_PORT:-3050}:8080"\n    depends_on:\n      mw-bff:\n        condition: service_healthy\n      mw-identity:\n        condition: service_healthy\n    networks:\n      - control-plane\n    healthcheck:\n      test:\n        [\n          "CMD-SHELL",\n          "wget -qO- http://127.0.0.1:8080/_gateway-health >/dev/null 2>&1",\n        ]\n      interval: 10s\n      timeout: 3s\n      retries: 5\n      start_period: 3s\n    restart: unless-stopped\n\n  mw-bff:\n    build:\n      context: ./mw-bff\n    environment:\n      APP_ENV: "${APP_ENV:-development}"\n      PORT: "7200"\n      DATABASE_PATH: "/app/data/bff.db"\n    expose:\n      - "7200"\n    volumes:\n      - bff-data:/app/data\n    networks:\n      - control-plane\n    healthcheck:\n      test:\n        [\n          "CMD-SHELL",\n          "wget -qO- http://127.0.0.1:7200/health >/dev/null 2>&1",\n        ]\n      interval: 10s\n      timeout: 3s\n      retries: 5\n      start_period: 3s\n    restart: unless-stopped\n\n  mw-identity:\n    build:\n      context: ./mw-identity\n    environment:\n      APP_ENV: "${APP_ENV:-development}"\n      PORT: "7100"\n      DATABASE_PATH: "/app/data/identity.db"\n      PUBLIC_ISSUER: "https://identity.mattediworks.com"\n    expose:\n      - "7100"\n    volumes:\n      - identity-data:/app/data\n    networks:\n      - control-plane\n    healthcheck:\n      test:\n        [\n          "CMD-SHELL",\n          "wget -qO- http://127.0.0.1:7100/health >/dev/null 2>&1",\n        ]\n      interval: 10s\n      timeout: 3s\n      retries: 5\n      start_period: 3s\n    restart: unless-stopped\n\nnetworks:\n  control-plane:\n    driver: bridge\n\nvolumes:\n  identity-data:\n  bff-data:\n', 'infra/gateway/Dockerfile': 'FROM nginx:alpine\n\nCOPY infra/gateway/nginx.conf /etc/nginx/nginx.conf\nCOPY infra/gateway/static/ /usr/share/nginx/html/\n', 'infra/gateway/nginx.conf': 'worker_processes auto;\n\nevents {\n    worker_connections 1024;\n}\n\nhttp {\n    include /etc/nginx/mime.types;\n    default_type application/octet-stream;\n\n    sendfile on;\n    server_tokens off;\n\n    log_format main\n        \'$remote_addr - $host [$time_local] \'\n        \'"$request_method $uri $server_protocol" \'\n        \'$status $body_bytes_sent \'\n        \'rt=$request_time\';\n\n    access_log /var/log/nginx/access.log main;\n    error_log /var/log/nginx/error.log warn;\n\n    map $http_x_forwarded_proto $mw_forwarded_proto {\n        default $http_x_forwarded_proto;\n        ""      $scheme;\n    }\n\n    map $http_x_real_ip $mw_real_ip {\n        default $http_x_real_ip;\n        ""      $remote_addr;\n    }\n\n    upstream mw_bff {\n        server mw-bff:7200;\n        keepalive 16;\n    }\n\n    upstream mw_identity {\n        server mw-identity:7100;\n        keepalive 16;\n    }\n\n    server {\n        listen 8080 default_server;\n        server_name _;\n\n        location = /_gateway-health {\n            access_log off;\n            add_header Content-Type text/plain;\n            return 200 "ok\\n";\n        }\n\n        location / {\n            return 404;\n        }\n    }\n\n    server {\n        listen 8080;\n        server_name admin.mattediworks.com;\n\n        root /usr/share/nginx/html;\n        index index.html;\n\n        add_header X-Content-Type-Options nosniff always;\n        add_header Referrer-Policy no-referrer always;\n        add_header X-Frame-Options DENY always;\n\n        location /api/ {\n            proxy_http_version 1.1;\n            proxy_pass http://mw_bff/;\n\n            proxy_set_header Host $host;\n            proxy_set_header X-Real-IP $mw_real_ip;\n            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n            proxy_set_header X-Forwarded-Proto $mw_forwarded_proto;\n\n            proxy_connect_timeout 3s;\n            proxy_send_timeout 15s;\n            proxy_read_timeout 15s;\n        }\n\n        location / {\n            try_files $uri $uri/ /index.html;\n        }\n    }\n\n    server {\n        listen 8080;\n        server_name identity.mattediworks.com;\n\n        add_header X-Content-Type-Options nosniff always;\n        add_header Referrer-Policy no-referrer always;\n        add_header X-Frame-Options DENY always;\n\n        location / {\n            proxy_http_version 1.1;\n            proxy_pass http://mw_identity;\n\n            proxy_set_header Host $host;\n            proxy_set_header X-Real-IP $mw_real_ip;\n            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n            proxy_set_header X-Forwarded-Proto $mw_forwarded_proto;\n\n            proxy_connect_timeout 3s;\n            proxy_send_timeout 15s;\n            proxy_read_timeout 15s;\n        }\n    }\n}\n', 'infra/gateway/static/index.html': '<!doctype html>\n<html lang="en">\n  <head>\n    <meta charset="UTF-8" />\n    <meta name="viewport" content="width=device-width, initial-scale=1.0" />\n    <title>MW Plane Control</title>\n  </head>\n  <body>\n    <main>\n      <h1>MW Plane Control Admin Test</h1>\n      <p>Step 01 gateway is running.</p>\n      <p>The real MW Web Admin application will replace this placeholder.</p>\n    </main>\n  </body>\n</html>\n', 'mw-identity/internal/buildinfo/buildinfo.go': 'package buildinfo\n\ntype Info struct {\n    Version string `json:"version"`\n    Commit  string `json:"commit"`\n    BuiltAt string `json:"built_at"`\n}\n\nvar (\n    Version = "dev"\n    Commit  = "none"\n    BuiltAt = "unknown"\n)\n\nfunc Current() Info {\n    return Info{\n        Version: Version,\n        Commit:  Commit,\n        BuiltAt: BuiltAt,\n    }\n}\n', 'mw-identity/internal/config/config.go': '    package config\n\n    import (\n        "fmt"\n        "os"\n        "strconv"\n        "strings"\n    )\n\n    type Config struct {\n        AppEnv       string\n        Port         int\n        DatabasePath string\n    \tPublicIssuer string\n    }\n\n    func Load() (Config, error) {\n        portRaw := getenv("PORT", "7100")\n        port, err := strconv.Atoi(portRaw)\n        if err != nil || port < 1 || port > 65535 {\n            return Config{}, fmt.Errorf("invalid PORT %q", portRaw)\n        }\n\n        databasePath := strings.TrimSpace(\n            getenv("DATABASE_PATH", "./data/identity.db"),\n        )\n        if databasePath == "" {\n            return Config{}, fmt.Errorf("DATABASE_PATH is required")\n        }\n\n        return Config{\n            AppEnv:       getenv("APP_ENV", "development"),\n            Port:         port,\n            DatabasePath: databasePath,\n\t\tPublicIssuer: getenv("PUBLIC_ISSUER", "https://identity.mattediworks.com"),\n        }, nil\n    }\n\n    func getenv(key, fallback string) string {\n        value, ok := os.LookupEnv(key)\n        if !ok {\n            return fallback\n        }\n\n        value = strings.TrimSpace(value)\n        if value == "" {\n            return fallback\n        }\n\n        return value\n    }\n', 'mw-identity/internal/config/config_test.go': '    package config\n\n    import "testing"\n\n    func TestLoadDefaults(t *testing.T) {\n        t.Setenv("PORT", "")\n        t.Setenv("DATABASE_PATH", "")\n        t.Setenv("APP_ENV", "")\n            t.Setenv("PUBLIC_ISSUER", "")\n\n        cfg, err := Load()\n        if err != nil {\n            t.Fatal(err)\n        }\n\n        if cfg.Port != 7100 {\n            t.Fatalf("Port = %d", cfg.Port)\n        }\n        if cfg.DatabasePath != "./data/identity.db" {\n            t.Fatalf("DatabasePath = %q", cfg.DatabasePath)\n        }\n    if cfg.PublicIssuer != "https://identity.mattediworks.com" {\n    t.Fatalf("PublicIssuer = %q", cfg.PublicIssuer)\n}\n    }\n\n    func TestLoadRejectsInvalidPort(t *testing.T) {\n        t.Setenv("PORT", "not-a-port")\n\n        if _, err := Load(); err == nil {\n            t.Fatal("expected invalid PORT error")\n        }\n    }\n', 'mw-identity/internal/store/sqlite/sqlite.go': 'package sqlite\n\nimport (\n    "context"\n    "database/sql"\n    "fmt"\n    "os"\n    "path/filepath"\n    "strings"\n    "time"\n\n    _ "modernc.org/sqlite"\n)\n\nfunc Open(path string) (*sql.DB, error) {\n    path = strings.TrimSpace(path)\n    if path == "" {\n        return nil, fmt.Errorf("sqlite path is required")\n    }\n\n    if path != ":memory:" && !strings.HasPrefix(path, "file:") {\n        dir := filepath.Dir(path)\n        if dir != "." {\n            if err := os.MkdirAll(dir, 0o700); err != nil {\n                return nil, fmt.Errorf("create sqlite directory: %w", err)\n            }\n        }\n    }\n\n    db, err := sql.Open("sqlite", path)\n    if err != nil {\n        return nil, fmt.Errorf("open sqlite: %w", err)\n    }\n\n    // A single connection makes the connection-scoped PRAGMAs below\n    // deterministic. This is adequate for the small Step 01 control-plane\n    // baseline and can be revisited after real concurrency measurements.\n    db.SetMaxOpenConns(1)\n    db.SetMaxIdleConns(1)\n\n    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n    defer cancel()\n\n    pragmas := []string{\n        "PRAGMA foreign_keys = ON",\n        "PRAGMA busy_timeout = 5000",\n        "PRAGMA journal_mode = WAL",\n    }\n\n    for _, pragma := range pragmas {\n        if _, err := db.ExecContext(ctx, pragma); err != nil {\n            _ = db.Close()\n            return nil, fmt.Errorf("%s: %w", pragma, err)\n        }\n    }\n\n    if err := db.PingContext(ctx); err != nil {\n        _ = db.Close()\n        return nil, fmt.Errorf("ping sqlite: %w", err)\n    }\n\n    return db, nil\n}\n', 'mw-identity/internal/store/sqlite/migrations.go': 'package sqlite\n\nimport (\n    "context"\n    "database/sql"\n    "fmt"\n    "io/fs"\n    "sort"\n    "strings"\n)\n\nfunc ApplyMigrations(\n    ctx context.Context,\n    db *sql.DB,\n    migrationFS fs.FS,\n) error {\n    if _, err := db.ExecContext(ctx, `\n        CREATE TABLE IF NOT EXISTS schema_migrations (\n            name TEXT PRIMARY KEY,\n            applied_at TEXT NOT NULL\n        )\n    `); err != nil {\n        return fmt.Errorf("create schema_migrations: %w", err)\n    }\n\n    entries, err := fs.ReadDir(migrationFS, ".")\n    if err != nil {\n        return fmt.Errorf("read migrations: %w", err)\n    }\n\n    names := make([]string, 0, len(entries))\n    for _, entry := range entries {\n        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {\n            continue\n        }\n        names = append(names, entry.Name())\n    }\n    sort.Strings(names)\n\n    for _, name := range names {\n        var applied int\n        err := db.QueryRowContext(\n            ctx,\n            `SELECT COUNT(1) FROM schema_migrations WHERE name = ?`,\n            name,\n        ).Scan(&applied)\n        if err != nil {\n            return fmt.Errorf("check migration %s: %w", name, err)\n        }\n        if applied != 0 {\n            continue\n        }\n\n        body, err := fs.ReadFile(migrationFS, name)\n        if err != nil {\n            return fmt.Errorf("read migration %s: %w", name, err)\n        }\n\n        tx, err := db.BeginTx(ctx, nil)\n        if err != nil {\n            return fmt.Errorf("begin migration %s: %w", name, err)\n        }\n\n        if _, err := tx.ExecContext(ctx, string(body)); err != nil {\n            _ = tx.Rollback()\n            return fmt.Errorf("apply migration %s: %w", name, err)\n        }\n\n        if _, err := tx.ExecContext(\n            ctx,\n            `INSERT INTO schema_migrations (name, applied_at)\n             VALUES (?, strftime(\'%Y-%m-%dT%H:%M:%fZ\', \'now\'))`,\n            name,\n        ); err != nil {\n            _ = tx.Rollback()\n            return fmt.Errorf("record migration %s: %w", name, err)\n        }\n\n        if err := tx.Commit(); err != nil {\n            return fmt.Errorf("commit migration %s: %w", name, err)\n        }\n    }\n\n    return nil\n}\n', 'mw-identity/internal/store/sqlite/sqlite_test.go': 'package sqlite\n\nimport (\n    "context"\n    "path/filepath"\n    "testing"\n\n    "github.com/vtmattedi/mw-plane-control/mw-identity/migrations"\n)\n\nfunc TestApplyMigrationsIsIdempotent(t *testing.T) {\n    db, err := Open(filepath.Join(t.TempDir(), "test.db"))\n    if err != nil {\n        t.Fatal(err)\n    }\n    defer db.Close()\n\n    ctx := context.Background()\n\n    if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {\n        t.Fatal(err)\n    }\n    if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {\n        t.Fatal(err)\n    }\n\n    var count int\n    if err := db.QueryRowContext(\n        ctx,\n        `SELECT COUNT(1) FROM schema_migrations`,\n    ).Scan(&count); err != nil {\n        t.Fatal(err)\n    }\n\n    if count != 1 {\n        t.Fatalf("migration count = %d; want 1", count)\n    }\n}\n', 'mw-identity/internal/httpapi/health.go': 'package httpapi\n\nimport (\n    "context"\n    "encoding/json"\n    "net/http"\n    "time"\n\n    "github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo"\n)\n\ntype pinger interface {\n    PingContext(context.Context) error\n}\n\ntype healthResponse struct {\n    Status   string         `json:"status"`\n    Service  string         `json:"service"`\n    Database string         `json:"database"`\n    Build    buildinfo.Info `json:"build"`\n}\n\nfunc healthHandler(\n    service string,\n    db pinger,\n) http.HandlerFunc {\n    return func(w http.ResponseWriter, r *http.Request) {\n        ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)\n        defer cancel()\n\n        response := healthResponse{\n            Status:   "ok",\n            Service:  service,\n            Database: "ok",\n            Build:    buildinfo.Current(),\n        }\n\n        status := http.StatusOK\n        if err := db.PingContext(ctx); err != nil {\n            response.Status = "unavailable"\n            response.Database = "unavailable"\n            status = http.StatusServiceUnavailable\n        }\n\n        w.Header().Set("Content-Type", "application/json")\n        w.Header().Set("Cache-Control", "no-store")\n        w.WriteHeader(status)\n        _ = json.NewEncoder(w).Encode(response)\n    }\n}\n', 'mw-identity/internal/httpapi/health_test.go': 'package httpapi\n\nimport (\n    "context"\n    "errors"\n    "net/http"\n    "net/http/httptest"\n    "testing"\n)\n\ntype fakePinger struct {\n    err error\n}\n\nfunc (p fakePinger) PingContext(context.Context) error {\n    return p.err\n}\n\nfunc TestHealthOK(t *testing.T) {\n    req := httptest.NewRequest(http.MethodGet, "/health", nil)\n    rec := httptest.NewRecorder()\n\n    healthHandler("test-service", fakePinger{}).ServeHTTP(rec, req)\n\n    if rec.Code != http.StatusOK {\n        t.Fatalf("status = %d", rec.Code)\n    }\n    if got := rec.Header().Get("Cache-Control"); got != "no-store" {\n        t.Fatalf("Cache-Control = %q", got)\n    }\n}\n\nfunc TestHealthUnavailable(t *testing.T) {\n    req := httptest.NewRequest(http.MethodGet, "/health", nil)\n    rec := httptest.NewRecorder()\n\n    healthHandler(\n        "test-service",\n        fakePinger{err: errors.New("db down")},\n    ).ServeHTTP(rec, req)\n\n    if rec.Code != http.StatusServiceUnavailable {\n        t.Fatalf("status = %d", rec.Code)\n    }\n}\n', 'mw-identity/internal/httpapi/security_headers.go': 'package httpapi\n\nimport "net/http"\n\nfunc securityHeaders(next http.Handler) http.Handler {\n    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n        w.Header().Set("X-Content-Type-Options", "nosniff")\n        w.Header().Set("Referrer-Policy", "no-referrer")\n        w.Header().Set("X-Frame-Options", "DENY")\n        next.ServeHTTP(w, r)\n    })\n}\n', 'mw-identity/internal/httpapi/router.go': 'package httpapi\n\nimport (\n    "database/sql"\n    "net/http"\n\n    "github.com/vtmattedi/mw-plane-control/mw-identity/internal/portal"\n)\n\nfunc New(db *sql.DB) http.Handler {\n    mux := http.NewServeMux()\n    mux.HandleFunc("GET /health", healthHandler("mw-identity", db))\n    mux.Handle("GET /{$}", portal.Handler())\n\n    return securityHeaders(mux)\n}\n', 'mw-identity/internal/portal/handler.go': 'package portal\n\nimport (\n    "net/http"\n\n    portalstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"\n)\n\nfunc Handler() http.Handler {\n    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n        w.Header().Set("Content-Type", "text/html; charset=utf-8")\n        w.Header().Set("Cache-Control", "no-store")\n        _, _ = w.Write(portalstatic.IndexHTML)\n    })\n}\n', 'mw-identity/web/static/embed.go': 'package static\n\nimport _ "embed"\n\n//go:embed index.html\nvar IndexHTML []byte\n', 'mw-identity/web/static/index.html': '<!doctype html>\n<html lang="en">\n  <head>\n    <meta charset="UTF-8" />\n    <meta name="viewport" content="width=device-width, initial-scale=1.0" />\n    <title>MW Identity</title>\n  </head>\n  <body>\n    <main>\n      <h1>MW Identity</h1>\n      <p>Identity service is running.</p>\n      <p>Authentication UI will be implemented in later steps.</p>\n    </main>\n  </body>\n</html>\n', 'mw-identity/migrations/embed.go': 'package migrations\n\nimport "embed"\n\n// FS contains the ordered SQL migrations for this service.\n//\n//go:embed *.sql\nvar FS embed.FS\n', 'mw-identity/migrations/001_bootstrap.sql': "CREATE TABLE service_metadata (\n    key TEXT PRIMARY KEY,\n    value TEXT NOT NULL,\n    updated_at TEXT NOT NULL\n);\n\nINSERT INTO service_metadata (key, value, updated_at)\nVALUES (\n    'schema.bootstrap',\n    'step-01',\n    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')\n);\n", 'mw-identity/cmd/server/main.go': '    package main\n\n    import (\n        "context"\n        "fmt"\n        "log/slog"\n        "net/http"\n        "os"\n        "os/signal"\n        "syscall"\n        "time"\n\n        "github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"\n        "github.com/vtmattedi/mw-plane-control/mw-identity/internal/httpapi"\n        storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"\n        "github.com/vtmattedi/mw-plane-control/mw-identity/migrations"\n    )\n\n    func main() {\n        logger := slog.New(\n            slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),\n        )\n        slog.SetDefault(logger)\n\n        cfg, err := config.Load()\n        if err != nil {\n            slog.Error("configuration failed", "error", err)\n            os.Exit(1)\n        }\n\n        db, err := storesqlite.Open(cfg.DatabasePath)\n        if err != nil {\n            slog.Error("database open failed", "error", err)\n            os.Exit(1)\n        }\n        defer db.Close()\n\n        migrationCtx, cancelMigrations := context.WithTimeout(\n            context.Background(),\n            10*time.Second,\n        )\n        if err := storesqlite.ApplyMigrations(\n            migrationCtx,\n            db,\n            migrations.FS,\n        ); err != nil {\n            cancelMigrations()\n            slog.Error("database migration failed", "error", err)\n            os.Exit(1)\n        }\n        cancelMigrations()\n\n        server := &http.Server{\n            Addr:              fmt.Sprintf(":%d", cfg.Port),\n            Handler:           httpapi.New(db),\n            ReadHeaderTimeout: 5 * time.Second,\n            ReadTimeout:       15 * time.Second,\n            WriteTimeout:      15 * time.Second,\n            IdleTimeout:       60 * time.Second,\n        }\n\n        shutdownCtx, stop := signal.NotifyContext(\n            context.Background(),\n            os.Interrupt,\n            syscall.SIGTERM,\n        )\n        defer stop()\n\n        go func() {\n            <-shutdownCtx.Done()\n\n            ctx, cancel := context.WithTimeout(\n                context.Background(),\n                10*time.Second,\n            )\n            defer cancel()\n\n            if err := server.Shutdown(ctx); err != nil {\n                slog.Error("server shutdown failed", "error", err)\n            }\n        }()\n\n        slog.Info(\n            "server starting",\n            "service", "mw-identity",\n            "app_env", cfg.AppEnv,\n            "port", cfg.Port,\n\t\t"public_issuer", cfg.PublicIssuer,\n        )\n\n        if err := server.ListenAndServe(); err != nil &&\n            err != http.ErrServerClosed {\n            slog.Error("server stopped unexpectedly", "error", err)\n            os.Exit(1)\n        }\n\n        slog.Info("server stopped", "service", "mw-identity")\n    }\n', 'mw-identity/Dockerfile': 'FROM golang:1.26-alpine AS build\n\nWORKDIR /src\n\nCOPY go.mod go.sum ./\nRUN go mod download\n\nCOPY . .\n\nARG VERSION=dev\nARG COMMIT=none\nARG BUILT_AT=unknown\n\nRUN CGO_ENABLED=0 GOOS=linux go build \\\n    -trimpath \\\n    -ldflags="-s -w \\\n    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.Version=${VERSION} \\\n    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.Commit=${COMMIT} \\\n    -X github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo.BuiltAt=${BUILT_AT}" \\\n    -o /out/server \\\n    ./cmd/server\n\nFROM alpine:3.22\n\nRUN apk add --no-cache ca-certificates tzdata \\\n    && addgroup -S app \\\n    && adduser -S -G app app \\\n    && mkdir -p /app/data \\\n    && chown -R app:app /app\n\nWORKDIR /app\n\nCOPY --from=build /out/server /app/server\n\nUSER app\n\nEXPOSE 7100\n\nENTRYPOINT ["/app/server"]\n', 'mw-identity/.dockerignore': '.git\n.backup\ndata\n*.db\n*.db-shm\n*.db-wal\n', 'mw-bff/internal/buildinfo/buildinfo.go': 'package buildinfo\n\ntype Info struct {\n    Version string `json:"version"`\n    Commit  string `json:"commit"`\n    BuiltAt string `json:"built_at"`\n}\n\nvar (\n    Version = "dev"\n    Commit  = "none"\n    BuiltAt = "unknown"\n)\n\nfunc Current() Info {\n    return Info{\n        Version: Version,\n        Commit:  Commit,\n        BuiltAt: BuiltAt,\n    }\n}\n', 'mw-bff/internal/config/config.go': 'package config\n\nimport (\n    "fmt"\n    "os"\n    "strconv"\n    "strings"\n)\n\ntype Config struct {\n    AppEnv       string\n    Port         int\n    DatabasePath string\n\n}\n\nfunc Load() (Config, error) {\n    portRaw := getenv("PORT", "7200")\n    port, err := strconv.Atoi(portRaw)\n    if err != nil || port < 1 || port > 65535 {\n        return Config{}, fmt.Errorf("invalid PORT %q", portRaw)\n    }\n\n    databasePath := strings.TrimSpace(\n        getenv("DATABASE_PATH", "./data/bff.db"),\n    )\n    if databasePath == "" {\n        return Config{}, fmt.Errorf("DATABASE_PATH is required")\n    }\n\n    return Config{\n        AppEnv:       getenv("APP_ENV", "development"),\n        Port:         port,\n        DatabasePath: databasePath,\n    }, nil\n}\n\nfunc getenv(key, fallback string) string {\n    value, ok := os.LookupEnv(key)\n    if !ok {\n        return fallback\n    }\n\n    value = strings.TrimSpace(value)\n    if value == "" {\n        return fallback\n    }\n\n    return value\n}\n', 'mw-bff/internal/config/config_test.go': 'package config\n\nimport "testing"\n\nfunc TestLoadDefaults(t *testing.T) {\n    t.Setenv("PORT", "")\n    t.Setenv("DATABASE_PATH", "")\n    t.Setenv("APP_ENV", "")\n\n\n    cfg, err := Load()\n    if err != nil {\n        t.Fatal(err)\n    }\n\n    if cfg.Port != 7200 {\n        t.Fatalf("Port = %d", cfg.Port)\n    }\n    if cfg.DatabasePath != "./data/bff.db" {\n        t.Fatalf("DatabasePath = %q", cfg.DatabasePath)\n    }\n\n}\n\nfunc TestLoadRejectsInvalidPort(t *testing.T) {\n    t.Setenv("PORT", "not-a-port")\n\n    if _, err := Load(); err == nil {\n        t.Fatal("expected invalid PORT error")\n    }\n}\n', 'mw-bff/internal/store/sqlite/sqlite.go': 'package sqlite\n\nimport (\n    "context"\n    "database/sql"\n    "fmt"\n    "os"\n    "path/filepath"\n    "strings"\n    "time"\n\n    _ "modernc.org/sqlite"\n)\n\nfunc Open(path string) (*sql.DB, error) {\n    path = strings.TrimSpace(path)\n    if path == "" {\n        return nil, fmt.Errorf("sqlite path is required")\n    }\n\n    if path != ":memory:" && !strings.HasPrefix(path, "file:") {\n        dir := filepath.Dir(path)\n        if dir != "." {\n            if err := os.MkdirAll(dir, 0o700); err != nil {\n                return nil, fmt.Errorf("create sqlite directory: %w", err)\n            }\n        }\n    }\n\n    db, err := sql.Open("sqlite", path)\n    if err != nil {\n        return nil, fmt.Errorf("open sqlite: %w", err)\n    }\n\n    // A single connection makes the connection-scoped PRAGMAs below\n    // deterministic. This is adequate for the small Step 01 control-plane\n    // baseline and can be revisited after real concurrency measurements.\n    db.SetMaxOpenConns(1)\n    db.SetMaxIdleConns(1)\n\n    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n    defer cancel()\n\n    pragmas := []string{\n        "PRAGMA foreign_keys = ON",\n        "PRAGMA busy_timeout = 5000",\n        "PRAGMA journal_mode = WAL",\n    }\n\n    for _, pragma := range pragmas {\n        if _, err := db.ExecContext(ctx, pragma); err != nil {\n            _ = db.Close()\n            return nil, fmt.Errorf("%s: %w", pragma, err)\n        }\n    }\n\n    if err := db.PingContext(ctx); err != nil {\n        _ = db.Close()\n        return nil, fmt.Errorf("ping sqlite: %w", err)\n    }\n\n    return db, nil\n}\n', 'mw-bff/internal/store/sqlite/migrations.go': 'package sqlite\n\nimport (\n    "context"\n    "database/sql"\n    "fmt"\n    "io/fs"\n    "sort"\n    "strings"\n)\n\nfunc ApplyMigrations(\n    ctx context.Context,\n    db *sql.DB,\n    migrationFS fs.FS,\n) error {\n    if _, err := db.ExecContext(ctx, `\n        CREATE TABLE IF NOT EXISTS schema_migrations (\n            name TEXT PRIMARY KEY,\n            applied_at TEXT NOT NULL\n        )\n    `); err != nil {\n        return fmt.Errorf("create schema_migrations: %w", err)\n    }\n\n    entries, err := fs.ReadDir(migrationFS, ".")\n    if err != nil {\n        return fmt.Errorf("read migrations: %w", err)\n    }\n\n    names := make([]string, 0, len(entries))\n    for _, entry := range entries {\n        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {\n            continue\n        }\n        names = append(names, entry.Name())\n    }\n    sort.Strings(names)\n\n    for _, name := range names {\n        var applied int\n        err := db.QueryRowContext(\n            ctx,\n            `SELECT COUNT(1) FROM schema_migrations WHERE name = ?`,\n            name,\n        ).Scan(&applied)\n        if err != nil {\n            return fmt.Errorf("check migration %s: %w", name, err)\n        }\n        if applied != 0 {\n            continue\n        }\n\n        body, err := fs.ReadFile(migrationFS, name)\n        if err != nil {\n            return fmt.Errorf("read migration %s: %w", name, err)\n        }\n\n        tx, err := db.BeginTx(ctx, nil)\n        if err != nil {\n            return fmt.Errorf("begin migration %s: %w", name, err)\n        }\n\n        if _, err := tx.ExecContext(ctx, string(body)); err != nil {\n            _ = tx.Rollback()\n            return fmt.Errorf("apply migration %s: %w", name, err)\n        }\n\n        if _, err := tx.ExecContext(\n            ctx,\n            `INSERT INTO schema_migrations (name, applied_at)\n             VALUES (?, strftime(\'%Y-%m-%dT%H:%M:%fZ\', \'now\'))`,\n            name,\n        ); err != nil {\n            _ = tx.Rollback()\n            return fmt.Errorf("record migration %s: %w", name, err)\n        }\n\n        if err := tx.Commit(); err != nil {\n            return fmt.Errorf("commit migration %s: %w", name, err)\n        }\n    }\n\n    return nil\n}\n', 'mw-bff/internal/store/sqlite/sqlite_test.go': 'package sqlite\n\nimport (\n    "context"\n    "path/filepath"\n    "testing"\n\n    "github.com/vtmattedi/mw-plane-control/mw-bff/migrations"\n)\n\nfunc TestApplyMigrationsIsIdempotent(t *testing.T) {\n    db, err := Open(filepath.Join(t.TempDir(), "test.db"))\n    if err != nil {\n        t.Fatal(err)\n    }\n    defer db.Close()\n\n    ctx := context.Background()\n\n    if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {\n        t.Fatal(err)\n    }\n    if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {\n        t.Fatal(err)\n    }\n\n    var count int\n    if err := db.QueryRowContext(\n        ctx,\n        `SELECT COUNT(1) FROM schema_migrations`,\n    ).Scan(&count); err != nil {\n        t.Fatal(err)\n    }\n\n    if count != 1 {\n        t.Fatalf("migration count = %d; want 1", count)\n    }\n}\n', 'mw-bff/internal/httpapi/health.go': 'package httpapi\n\nimport (\n    "context"\n    "encoding/json"\n    "net/http"\n    "time"\n\n    "github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo"\n)\n\ntype pinger interface {\n    PingContext(context.Context) error\n}\n\ntype healthResponse struct {\n    Status   string         `json:"status"`\n    Service  string         `json:"service"`\n    Database string         `json:"database"`\n    Build    buildinfo.Info `json:"build"`\n}\n\nfunc healthHandler(\n    service string,\n    db pinger,\n) http.HandlerFunc {\n    return func(w http.ResponseWriter, r *http.Request) {\n        ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)\n        defer cancel()\n\n        response := healthResponse{\n            Status:   "ok",\n            Service:  service,\n            Database: "ok",\n            Build:    buildinfo.Current(),\n        }\n\n        status := http.StatusOK\n        if err := db.PingContext(ctx); err != nil {\n            response.Status = "unavailable"\n            response.Database = "unavailable"\n            status = http.StatusServiceUnavailable\n        }\n\n        w.Header().Set("Content-Type", "application/json")\n        w.Header().Set("Cache-Control", "no-store")\n        w.WriteHeader(status)\n        _ = json.NewEncoder(w).Encode(response)\n    }\n}\n', 'mw-bff/internal/httpapi/health_test.go': 'package httpapi\n\nimport (\n    "context"\n    "errors"\n    "net/http"\n    "net/http/httptest"\n    "testing"\n)\n\ntype fakePinger struct {\n    err error\n}\n\nfunc (p fakePinger) PingContext(context.Context) error {\n    return p.err\n}\n\nfunc TestHealthOK(t *testing.T) {\n    req := httptest.NewRequest(http.MethodGet, "/health", nil)\n    rec := httptest.NewRecorder()\n\n    healthHandler("test-service", fakePinger{}).ServeHTTP(rec, req)\n\n    if rec.Code != http.StatusOK {\n        t.Fatalf("status = %d", rec.Code)\n    }\n    if got := rec.Header().Get("Cache-Control"); got != "no-store" {\n        t.Fatalf("Cache-Control = %q", got)\n    }\n}\n\nfunc TestHealthUnavailable(t *testing.T) {\n    req := httptest.NewRequest(http.MethodGet, "/health", nil)\n    rec := httptest.NewRecorder()\n\n    healthHandler(\n        "test-service",\n        fakePinger{err: errors.New("db down")},\n    ).ServeHTTP(rec, req)\n\n    if rec.Code != http.StatusServiceUnavailable {\n        t.Fatalf("status = %d", rec.Code)\n    }\n}\n', 'mw-bff/internal/httpapi/security_headers.go': 'package httpapi\n\nimport "net/http"\n\nfunc securityHeaders(next http.Handler) http.Handler {\n    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n        w.Header().Set("X-Content-Type-Options", "nosniff")\n        w.Header().Set("Referrer-Policy", "no-referrer")\n        w.Header().Set("X-Frame-Options", "DENY")\n        next.ServeHTTP(w, r)\n    })\n}\n', 'mw-bff/internal/httpapi/router.go': 'package httpapi\n\nimport (\n    "database/sql"\n    "net/http"\n)\n\nfunc New(db *sql.DB) http.Handler {\n    mux := http.NewServeMux()\n    mux.HandleFunc("GET /health", healthHandler("mw-bff", db))\n\n    return securityHeaders(mux)\n}\n', 'mw-bff/migrations/embed.go': 'package migrations\n\nimport "embed"\n\n// FS contains the ordered SQL migrations for this service.\n//\n//go:embed *.sql\nvar FS embed.FS\n', 'mw-bff/migrations/001_bootstrap.sql': "CREATE TABLE service_metadata (\n    key TEXT PRIMARY KEY,\n    value TEXT NOT NULL,\n    updated_at TEXT NOT NULL\n);\n\nINSERT INTO service_metadata (key, value, updated_at)\nVALUES (\n    'schema.bootstrap',\n    'step-01',\n    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')\n);\n", 'mw-bff/cmd/server/main.go': 'package main\n\nimport (\n    "context"\n    "fmt"\n    "log/slog"\n    "net/http"\n    "os"\n    "os/signal"\n    "syscall"\n    "time"\n\n    "github.com/vtmattedi/mw-plane-control/mw-bff/internal/config"\n    "github.com/vtmattedi/mw-plane-control/mw-bff/internal/httpapi"\n    storesqlite "github.com/vtmattedi/mw-plane-control/mw-bff/internal/store/sqlite"\n    "github.com/vtmattedi/mw-plane-control/mw-bff/migrations"\n)\n\nfunc main() {\n    logger := slog.New(\n        slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),\n    )\n    slog.SetDefault(logger)\n\n    cfg, err := config.Load()\n    if err != nil {\n        slog.Error("configuration failed", "error", err)\n        os.Exit(1)\n    }\n\n    db, err := storesqlite.Open(cfg.DatabasePath)\n    if err != nil {\n        slog.Error("database open failed", "error", err)\n        os.Exit(1)\n    }\n    defer db.Close()\n\n    migrationCtx, cancelMigrations := context.WithTimeout(\n        context.Background(),\n        10*time.Second,\n    )\n    if err := storesqlite.ApplyMigrations(\n        migrationCtx,\n        db,\n        migrations.FS,\n    ); err != nil {\n        cancelMigrations()\n        slog.Error("database migration failed", "error", err)\n        os.Exit(1)\n    }\n    cancelMigrations()\n\n    server := &http.Server{\n        Addr:              fmt.Sprintf(":%d", cfg.Port),\n        Handler:           httpapi.New(db),\n        ReadHeaderTimeout: 5 * time.Second,\n        ReadTimeout:       15 * time.Second,\n        WriteTimeout:      15 * time.Second,\n        IdleTimeout:       60 * time.Second,\n    }\n\n    shutdownCtx, stop := signal.NotifyContext(\n        context.Background(),\n        os.Interrupt,\n        syscall.SIGTERM,\n    )\n    defer stop()\n\n    go func() {\n        <-shutdownCtx.Done()\n\n        ctx, cancel := context.WithTimeout(\n            context.Background(),\n            10*time.Second,\n        )\n        defer cancel()\n\n        if err := server.Shutdown(ctx); err != nil {\n            slog.Error("server shutdown failed", "error", err)\n        }\n    }()\n\n    slog.Info(\n        "server starting",\n        "service", "mw-bff",\n        "app_env", cfg.AppEnv,\n        "port", cfg.Port,\n    )\n\n    if err := server.ListenAndServe(); err != nil &&\n        err != http.ErrServerClosed {\n        slog.Error("server stopped unexpectedly", "error", err)\n        os.Exit(1)\n    }\n\n    slog.Info("server stopped", "service", "mw-bff")\n}\n', 'mw-bff/Dockerfile': 'FROM golang:1.26-alpine AS build\n\nWORKDIR /src\n\nCOPY go.mod go.sum ./\nRUN go mod download\n\nCOPY . .\n\nARG VERSION=dev\nARG COMMIT=none\nARG BUILT_AT=unknown\n\nRUN CGO_ENABLED=0 GOOS=linux go build \\\n    -trimpath \\\n    -ldflags="-s -w \\\n    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.Version=${VERSION} \\\n    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.Commit=${COMMIT} \\\n    -X github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo.BuiltAt=${BUILT_AT}" \\\n    -o /out/server \\\n    ./cmd/server\n\nFROM alpine:3.22\n\nRUN apk add --no-cache ca-certificates tzdata \\\n    && addgroup -S app \\\n    && adduser -S -G app app \\\n    && mkdir -p /app/data \\\n    && chown -R app:app /app\n\nWORKDIR /app\n\nCOPY --from=build /out/server /app/server\n\nUSER app\n\nEXPOSE 7200\n\nENTRYPOINT ["/app/server"]\n', 'mw-bff/.dockerignore': '.git\n.backup\ndata\n*.db\n*.db-shm\n*.db-wal\n', 'scripts/sanity-step-01.ps1': '\\\nparam(\n    [int]$Port = 3050\n)\n\n$ErrorActionPreference = "Stop"\n$Base = "http://127.0.0.1:$Port"\n\nfunction Invoke-CurlText {\n    param(\n        [string]$HostName,\n        [string]$Path\n    )\n\n    $text = & curl.exe `\n        --fail `\n        --silent `\n        --show-error `\n        -H "Host: $HostName" `\n        "$Base$Path"\n\n    if ($LASTEXITCODE -ne 0) {\n        throw "curl failed for $HostName$Path"\n    }\n\n    return ($text -join "`n")\n}\n\nWrite-Host "== Step 01 runtime sanity =="\n\n$admin = Invoke-CurlText `\n    -HostName "admin.mattediworks.com" `\n    -Path "/"\n\nif ($admin -notmatch "MW Plane Control Admin Test") {\n    throw "Admin placeholder did not come from the gateway static host."\n}\nWrite-Host "PASS admin static host"\n\n$bffRaw = Invoke-CurlText `\n    -HostName "admin.mattediworks.com" `\n    -Path "/api/health"\n$bff = $bffRaw | ConvertFrom-Json\n\nif ($bff.status -ne "ok" -or $bff.service -ne "mw-bff") {\n    throw "BFF health response was unexpected: $bffRaw"\n}\nWrite-Host "PASS admin /api/health -> MW BFF"\n\n$identityRaw = Invoke-CurlText `\n    -HostName "identity.mattediworks.com" `\n    -Path "/health"\n$identity = $identityRaw | ConvertFrom-Json\n\nif (\n    $identity.status -ne "ok" -or\n    $identity.service -ne "mw-identity"\n) {\n    throw "Identity health response was unexpected: $identityRaw"\n}\nWrite-Host "PASS identity /health -> MW Identity"\n\n$portal = Invoke-CurlText `\n    -HostName "identity.mattediworks.com" `\n    -Path "/"\n\nif ($portal -notmatch "MW Identity") {\n    throw "Identity portal HTML was unexpected."\n}\nWrite-Host "PASS identity portal"\n\n$status = & curl.exe `\n    --silent `\n    --output NUL `\n    --write-out "%{http_code}" `\n    -H "Host: unexpected.example" `\n    "$Base/"\n\nif ($LASTEXITCODE -ne 0) {\n    throw "curl failed while checking unknown host"\n}\n\nif ($status -ne "404") {\n    throw "Unknown host returned HTTP $status; expected 404."\n}\nWrite-Host "PASS unknown Host -> 404"\n\nWrite-Host ""\ndocker compose ps\n\nWrite-Host ""\nWrite-Host "STEP 01 E2E PASS"\n', 'scripts/e2e-step-01.ps1': '\\\nparam(\n    [int]$Port = 3050\n)\n\n$ErrorActionPreference = "Stop"\n\n$env:GATEWAY_HOST_PORT = "$Port"\n\nWrite-Host "== Build and start Step 01 =="\ndocker compose up -d --build\nif ($LASTEXITCODE -ne 0) {\n    throw "docker compose up failed"\n}\n\nWrite-Host ""\nWrite-Host "== Wait for gateway health =="\n\n$healthy = $false\nfor ($i = 0; $i -lt 30; $i++) {\n    try {\n        & curl.exe `\n            --fail `\n            --silent `\n            --show-error `\n            "http://127.0.0.1:$Port/_gateway-health" `\n            | Out-Null\n\n        if ($LASTEXITCODE -eq 0) {\n            $healthy = $true\n            break\n        }\n    } catch {\n    }\n\n    Start-Sleep -Seconds 1\n}\n\nif (-not $healthy) {\n    docker compose ps\n    docker compose logs --tail=100\n    throw "gateway did not become healthy in time"\n}\n\n& "$PSScriptRoot\\sanity-step-01.ps1" -Port $Port\n', 'scripts/sanity-step-01.sh': '#!/usr/bin/env sh\nset -eu\n\nPORT="${1:-3050}"\nBASE="http://127.0.0.1:${PORT}"\n\necho "== Step 01 runtime sanity =="\n\nadmin="$(curl -fsS -H \'Host: admin.mattediworks.com\' "${BASE}/")"\nprintf \'%s\' "$admin" | grep -q \'MW Plane Control Admin Test\'\necho "PASS admin static host"\n\nbff="$(curl -fsS -H \'Host: admin.mattediworks.com\' "${BASE}/api/health")"\nprintf \'%s\' "$bff" | grep -q \'"status":"ok"\'\nprintf \'%s\' "$bff" | grep -q \'"service":"mw-bff"\'\necho "PASS admin /api/health -> MW BFF"\n\nidentity="$(curl -fsS -H \'Host: identity.mattediworks.com\' "${BASE}/health")"\nprintf \'%s\' "$identity" | grep -q \'"status":"ok"\'\nprintf \'%s\' "$identity" | grep -q \'"service":"mw-identity"\'\necho "PASS identity /health -> MW Identity"\n\nportal="$(curl -fsS -H \'Host: identity.mattediworks.com\' "${BASE}/")"\nprintf \'%s\' "$portal" | grep -q \'MW Identity\'\necho "PASS identity portal"\n\nstatus="$(\n  curl -sS -o /dev/null -w \'%{http_code}\'     -H \'Host: unexpected.example\'     "${BASE}/"\n)"\ntest "$status" = "404"\necho "PASS unknown Host -> 404"\n\necho\ndocker compose ps\necho\necho "STEP 01 E2E PASS"\n', 'scripts/e2e-step-01.sh': '#!/usr/bin/env sh\nset -eu\n\nPORT="${1:-3050}"\nexport GATEWAY_HOST_PORT="$PORT"\n\necho "== Build and start Step 01 =="\ndocker compose up -d --build\n\necho\necho "== Wait for gateway health =="\n\ni=0\nwhile [ "$i" -lt 30 ]; do\n  if curl -fsS "http://127.0.0.1:${PORT}/_gateway-health" >/dev/null 2>&1; then\n    exec "$(dirname "$0")/sanity-step-01.sh" "$PORT"\n  fi\n  i=$((i + 1))\n  sleep 1\ndone\n\ndocker compose ps\ndocker compose logs --tail=100\necho "gateway did not become healthy in time" >&2\nexit 1\n', 'docs/step-01-runtime-skeleton.md': '# Step 01 — Gateway + BFF + Identity Health E2E\n\n## Implemented topology\n\n```text\n127.0.0.1:3050\n      |\n      v\ngateway:8080\n  |\n  +-- Host admin.mattediworks.com\n  |     +-- /api/* -> mw-bff:7200\n  |     +-- /*     -> static placeholder\n  |\n  +-- Host identity.mattediworks.com\n        +-- /*     -> mw-identity:7100\n```\n\nOnly gateway publishes a host port.\n\nBFF and Identity have independent SQLite databases on named volumes.\n\n## Identity\n\nStep 01 provides:\n\n```text\nGET /\nGET /health\n```\n\nThe root is deliberately plain HTML.\n\nThe service also has:\n\n- env config;\n- structured JSON logging;\n- graceful shutdown;\n- SQLite WAL/foreign keys/busy timeout;\n- embedded SQL migrations;\n- build metadata variables;\n- health check including database ping.\n\n## BFF\n\nStep 01 provides:\n\n```text\nGET /health\n```\n\nExternally through the Admin gateway this is:\n\n```text\nGET https://admin.mattediworks.com/api/health\n```\n\nThe BFF has its own independent SQLite database and migration history.\n\n## Gateway\n\nThe gateway:\n\n- strictly routes the two known Host values;\n- returns 404 for an unknown Host;\n- serves a temporary static Admin page;\n- strips `/api/` before proxying to BFF;\n- proxies the Identity host to MW Identity;\n- preserves Host and forwarded proxy headers;\n- preserves an outer `X-Forwarded-Proto` instead of replacing HTTPS with the\n  inner HTTP scheme.\n\nThe outer StackPort Nginx will terminate TLS later.\n\n## Static checks\n\nThe Step 01 generator runs:\n\n```text\ngo mod tidy\ngofmt\ngo test ./...\ngo vet ./...\ndocker compose config\n```\n\nfor the relevant projects.\n\n## Runtime E2E\n\nPowerShell:\n\n```powershell\n.\\scripts\\e2e-step-01.ps1\n```\n\nOptional port:\n\n```powershell\n.\\scripts\\e2e-step-01.ps1 -Port 3051\n```\n\nThe script leaves the stack running for inspection.\n\nTo stop it without deleting named SQLite volumes:\n\n```powershell\ndocker compose down\n```\n\nDo not use:\n\n```text\ndocker compose down -v\n```\n\nunless intentionally deleting Step 01 data.\n\n## Completion criteria\n\nStep 01 is complete only when the runtime sanity script reports:\n\n```text\nSTEP 01 E2E PASS\n```\n\nDo not begin Step 02 before that checkpoint is clean.\n'}

IDENTITY_MODULE = "github.com/vtmattedi/mw-plane-control/mw-identity"
BFF_MODULE = "github.com/vtmattedi/mw-plane-control/mw-bff"

EXPECTED_ROOT_FILES = [
    "README.md",
    ".gitignore",
    "docs/project.md",
    "docs/implementation-roadmap.md",
]

EXPECTED_DIRS = [
    "mw-identity",
    "mw-bff",
    "mw-web-admin",
    "infra/gateway",
]

STEP_MARKERS = [
    "docker-compose.yml",
    "mw-identity/go.mod",
    "mw-bff/go.mod",
    "infra/gateway/nginx.conf",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Implement MW Plane Control Step 01."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and print planned files without changing anything.",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help=(
            "Allow replacing an existing partial Step 01 implementation. "
            "Existing files are copied into .backup/ first."
        ),
    )
    parser.add_argument(
        "--skip-checks",
        action="store_true",
        help="Write files without running Go/Compose static checks.",
    )
    return parser.parse_args()


def fail(message: str) -> None:
    print(f"ERROR: {message}", file=sys.stderr)
    raise SystemExit(1)


def run(cmd: list[str], cwd: Path) -> None:
    print("+", " ".join(cmd))
    subprocess.run(cmd, cwd=cwd, check=True, text=True)


def check_root(root: Path, force: bool) -> None:
    for rel in EXPECTED_ROOT_FILES:
        if not (root / rel).is_file():
            fail(
                f"missing expected scaffold file: {rel}. "
                "Run this from the mw-plane-control root."
            )

    for rel in EXPECTED_DIRS:
        if not (root / rel).is_dir():
            fail(
                f"missing expected scaffold directory: {rel}. "
                "Run this from the mw-plane-control root."
            )

    existing_markers = [
        rel for rel in STEP_MARKERS if (root / rel).exists()
    ]

    if existing_markers and not force:
        fail(
            "Step 01 appears partially/already applied. "
            "Refusing to overwrite without --force. Existing markers: "
            + ", ".join(existing_markers)
        )


def patch_gitignore(text: str) -> str:
    lines = text.splitlines()

    normalized = []
    found = False

    for line in lines:
        if line.strip() in {".backups/", ".backup/"}:
            if not found:
                normalized.append(".backup/")
                found = True
            continue
        normalized.append(line)

    if not found:
        insert_at = 0
        while (
            insert_at < len(normalized)
            and normalized[insert_at].strip() == ""
        ):
            insert_at += 1
        normalized.insert(insert_at, ".backup/")

    return "\n".join(normalized).rstrip() + "\n"


def backup_file(
    root: Path,
    path: Path,
    backup_dir: Path,
) -> None:
    if not path.exists() or not path.is_file():
        return

    rel = path.relative_to(root)
    destination = backup_dir / rel
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, destination)


def write_file(
    root: Path,
    rel: str,
    content: str,
    backup_dir: Path,
) -> None:
    path = root / rel

    if path.exists() and path.is_file():
        current = path.read_text(encoding="utf-8")
        if current == content:
            print(f"unchanged: {rel}")
            return
        backup_file(root, path, backup_dir)

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8", newline="\n")
    print(f"wrote: {rel}")


def ensure_go_module(
    project: Path,
    module: str,
) -> None:
    go_mod = project / "go.mod"

    if not go_mod.exists():
        run(["go", "mod", "init", module], project)

    run(
        ["go", "get", "modernc.org/sqlite@latest"],
        project,
    )
    run(["go", "mod", "tidy"], project)


def gofmt_project(project: Path) -> None:
    go_files = sorted(
        str(path.relative_to(project))
        for path in project.rglob("*.go")
    )
    if go_files:
        run(["gofmt", "-w", *go_files], project)


def static_checks(root: Path) -> None:
    identity = root / "mw-identity"
    bff = root / "mw-bff"

    ensure_go_module(identity, IDENTITY_MODULE)
    ensure_go_module(bff, BFF_MODULE)

    gofmt_project(identity)
    gofmt_project(bff)

    print("\n== MW Identity checks ==")
    run(["go", "test", "./..."], identity)
    run(["go", "vet", "./..."], identity)

    print("\n== MW BFF checks ==")
    run(["go", "test", "./..."], bff)
    run(["go", "vet", "./..."], bff)

    print("\n== Compose validation ==")
    run(["docker", "compose", "config"], root)


def main() -> None:
    args = parse_args()
    root = Path.cwd().resolve()

    check_root(root, args.force)

    print("== MW Plane Control Step 01 ==")
    print("Gateway + BFF + Identity health E2E")
    print("")

    planned = sorted(FILES)
    planned.append(".gitignore")
    planned.extend(
        [
            "mw-identity/go.mod",
            "mw-identity/go.sum",
            "mw-bff/go.mod",
            "mw-bff/go.sum",
        ]
    )

    if args.dry_run:
        print("Planned writes:")
        for rel in sorted(set(planned)):
            print(f"  {rel}")
        print("")
        print("No files were changed.")
        return

    backup_root = root / ".backup"
    backup_root.mkdir(parents=True, exist_ok=True)
    stamp = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    backup_dir = backup_root / f"step-01-{stamp}"
    backup_dir.mkdir(parents=True, exist_ok=False)
    print(f"backup: {backup_dir.relative_to(root)}")

    gitignore = root / ".gitignore"
    old_gitignore = gitignore.read_text(encoding="utf-8")
    new_gitignore = patch_gitignore(old_gitignore)
    if new_gitignore != old_gitignore:
        backup_file(root, gitignore, backup_dir)
        gitignore.write_text(
            new_gitignore,
            encoding="utf-8",
            newline="\n",
        )
        print("patched: .gitignore -> .backup/")
    else:
        print("unchanged: .gitignore")

    for rel, content in sorted(FILES.items()):
        write_file(
            root,
            rel,
            content,
            backup_dir,
        )

    if os.name != "nt":
        for rel in (
            "scripts/sanity-step-01.sh",
            "scripts/e2e-step-01.sh",
        ):
            path = root / rel
            path.chmod(path.stat().st_mode | 0o111)

    if args.skip_checks:
        print("")
        print("Static checks skipped by request.")
    else:
        try:
            static_checks(root)
        except FileNotFoundError as exc:
            fail(f"required tool not found: {exc.filename}")
        except subprocess.CalledProcessError as exc:
            fail(
                "Step 01 static check failed with exit code "
                f"{exc.returncode}. Do not continue to Step 02."
            )

    print("")
    print("Step 01 implementation/static checks complete.")
    print("Runtime E2E is still required.")
    print("")
    if os.name == "nt":
        print("Run:")
        print(r"  .\scripts\e2e-step-01.ps1")
    else:
        print("Run:")
        print("  ./scripts/e2e-step-01.sh")
    print("")
    print(
        "Do not start Step 02 until the runtime script prints "
        "'STEP 01 E2E PASS'."
    )


if __name__ == "__main__":
    main()
