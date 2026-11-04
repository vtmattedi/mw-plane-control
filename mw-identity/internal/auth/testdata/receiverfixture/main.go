// receiverfixture drives the real deletion worker from a consumer's isolated
// integration suite. It never reads deployment configuration or an existing DB.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

const clientID = "woi-test"

type fixture struct {
	mu      sync.RWMutex
	path    string
	issuer  string
	keyring signing.Keyring
	db      *sql.DB
	store   *storesqlite.Store
	signer  *signing.Manager
	service *auth.Service
}

type command struct {
	Action      string `json:"action"`
	Callback    string `json:"callback"`
	Subject     string `json:"subject"`
	OperationID string `json:"operation_id"`
}

type result struct {
	Error          string                  `json:"error,omitempty"`
	Issuer         string                  `json:"issuer,omitempty"`
	Operation      *auth.DeletionOperation `json:"operation,omitempty"`
	SubjectExists  bool                    `json:"subject_exists"`
	SubjectEnabled bool                    `json:"subject_enabled"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	directory, err := os.MkdirTemp("", "mw-deletion-receiver-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	keyring, err := signing.ParseKeyring("1:"+base64.RawURLEncoding.EncodeToString(key), 1)
	if err != nil {
		return err
	}
	f := &fixture{path: filepath.Join(directory, "identity.db"), issuer: "http://" + listener.Addr().String(), keyring: keyring}
	if err := f.open(ctx); err != nil {
		return err
	}
	defer func() { f.db.Close() }()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oidc.Discovery(f.issuer))
	})
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		f.mu.RLock()
		defer f.mu.RUnlock()
		keys, err := f.signer.PublicJWKS(r.Context())
		if err != nil {
			http.Error(w, "test JWKS unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	defer server.Close()
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(result{Issuer: f.issuer}); err != nil {
		return err
	}
	decoder := json.NewDecoder(os.Stdin)
	for {
		var request command
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if request.Action == "close" {
			return encoder.Encode(result{})
		}
		response, err := f.execute(ctx, request)
		if err != nil {
			response = result{Error: err.Error()}
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
}

func (f *fixture) open(ctx context.Context) error {
	db, err := storesqlite.Open(f.path)
	if err != nil {
		return err
	}
	if err = storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		db.Close()
		return err
	}
	store := storesqlite.NewStore(db)
	manager, err := signing.NewManager(store, f.keyring)
	if err != nil {
		db.Close()
		return err
	}
	if _, err = manager.EnsureActiveKey(ctx); err != nil {
		db.Close()
		return err
	}
	service, err := auth.NewService(store, store, auth.ServiceConfig{SessionTTL: time.Hour, PublicIssuer: f.issuer})
	if err != nil {
		db.Close()
		return err
	}
	service.SetLifecycleSigner(manager)
	service.SetDurableDeletion(true, false) // This disposable test explicitly opts in.
	f.db, f.store, f.signer, f.service = db, store, manager, service
	return nil
}

func (f *fixture) execute(ctx context.Context, c command) (result, error) {
	switch c.Action {
	case "register":
		callback, err := url.Parse(c.Callback)
		if err != nil || callback.Scheme != "http" || callback.Hostname() != "127.0.0.1" || callback.User != nil {
			return result{}, errors.New("fixture callback must be local HTTP")
		}
		err = f.store.CreateApplication(ctx, applications.CreateInput{ID: "receiver-app", Name: "Receiver test", ClientID: clientID, ApplicationType: "public", AccessPolicy: "default_allow", AccessPolicyV1: true, AuthenticationPolicy: "standard", Enabled: true, DeleteCallbackURI: c.Callback, Scopes: []string{"openid"}, CreatedAt: time.Now().UTC()})
		return result{}, err
	case "create":
		if c.Subject == "" {
			return result{}, errors.New("subject required")
		}
		now := time.Now().UTC()
		err := f.store.CreateIdentityUser(ctx, auth.NewUser{ID: c.Subject, Email: c.Subject + "@example.test", EmailNormalized: c.Subject + "@example.test", DisplayName: "Receiver fixture", PasswordHash: "unused-test-only", Enabled: true, AccountType: "public", CreatedAt: now})
		if err != nil {
			return result{}, err
		}
		if err = f.store.GrantIdentityConsent(ctx, c.Subject, "receiver-app", []string{"openid"}, 0, now); err != nil {
			return result{}, err
		}
		deleted, err := f.service.DeleteAccount(ctx, c.Subject, "")
		if err != nil {
			return result{}, err
		}
		return f.status(ctx, deleted.OperationID)
	case "deliver":
		// Advance only this operation's test clock, without a backoff sleep.
		if _, err := f.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET next_attempt_at_unix=0 WHERE operation_id=? AND status='pending'`, c.OperationID); err != nil {
			return result{}, err
		}
		if _, err := f.service.ProcessDeletionDelivery(ctx); err != nil {
			return result{}, err
		}
		return f.status(ctx, c.OperationID)
	case "status":
		return f.status(ctx, c.OperationID)
	case "restart":
		f.mu.Lock()
		defer f.mu.Unlock()
		if err := f.db.Close(); err != nil {
			return result{}, err
		}
		return result{}, f.open(ctx)
	case "rotate":
		_, err := f.signer.Rotate(ctx)
		return result{}, err
	default:
		return result{}, errors.New("unsupported fixture command")
	}
}

func (f *fixture) status(ctx context.Context, id string) (result, error) {
	op, err := f.store.DeletionOperation(ctx, id)
	if err != nil {
		return result{}, err
	}
	user, err := f.store.IdentityUserByID(ctx, op.Subject)
	if err != nil && !errors.Is(err, auth.ErrNotFound) {
		return result{}, err
	}
	return result{Operation: &op, SubjectExists: err == nil, SubjectEnabled: err == nil && user.Enabled}, nil
}
