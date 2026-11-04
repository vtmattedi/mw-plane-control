package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	name := flag.String("name", "", "application name")
	clientID := flag.String("client-id", "", "OIDC client_id")
	appType := flag.String(
		"type",
		"confidential",
		"application type: confidential or public",
	)
	accessPolicy := flag.String(
		"access-policy",
		"default_deny",
		"application access policy: default_allow or default_deny",
	)
	deleteCallbackURI := flag.String(
		"delete-callback-uri",
		"",
		"HTTPS whole-MW-account deletion callback URI; required unless -internal",
	)
	internal := flag.Bool(
		"internal",
		false,
		"internal application behind the MW BFF with no user data of its own; exempt from the delete callback",
	)
	asJSON := flag.Bool(
		"json",
		false,
		"print machine-readable JSON",
	)

	var redirectURIs stringList
	var postLogoutURIs stringList
	var scopes stringList
	var claims stringList
	var audiences stringList

	flag.Var(
		&redirectURIs,
		"redirect-uri",
		"exact redirect URI; repeatable",
	)
	flag.Var(
		&postLogoutURIs,
		"post-logout-redirect-uri",
		"exact post-logout redirect URI; repeatable",
	)
	flag.Var(
		&scopes,
		"scope",
		"allowed scope; repeatable",
	)
	flag.Var(
		&claims,
		"claim",
		"allowed claim; repeatable",
	)
	flag.Var(
		&audiences,
		"audience",
		"allowed token audience; repeatable",
	)

	flag.Parse()

	if strings.TrimSpace(*name) == "" {
		fatal("name is required")
	}
	if strings.TrimSpace(*clientID) == "" {
		fatal("client-id is required")
	}
	if len(redirectURIs) == 0 {
		fatal("at least one redirect-uri is required")
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(
			fmt.Sprintf(
				"configuration failed: %v",
				err,
			),
		)
	}

	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(
			fmt.Sprintf(
				"database open failed: %v",
				err,
			),
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	if err := storesqlite.ApplyMigrations(
		ctx,
		db,
		migrations.FS,
	); err != nil {
		fatal(
			fmt.Sprintf(
				"database migration failed: %v",
				err,
			),
		)
	}

	service, err := applications.NewService(
		storesqlite.NewStore(db),
	)
	if err != nil {
		fatal(err.Error())
	}

	service.RequireHTTPS(cfg.AppEnv == "production")
	result, err := service.Create(
		ctx,
		applications.CreateRequest{
			Name:                   *name,
			ClientID:               *clientID,
			ApplicationType:        *appType,
			AccessPolicy:           *accessPolicy,
			RedirectURIs:           redirectURIs,
			PostLogoutRedirectURIs: postLogoutURIs,
			DeleteCallbackURI:      *deleteCallbackURI,
			Internal:               *internal,
			Scopes:                 scopes,
			Claims:                 claims,
			Audiences:              audiences,
		},
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			applications.ErrClientIDExists,
		):
			fatal("client-id already exists")
		case errors.Is(err, applications.ErrDeleteCallbackRequired):
			fatal("delete-callback-uri is required for applications that are not -internal")
		case errors.Is(
			err,
			applications.ErrInvalidApplication,
		):
			fatal("invalid application configuration")
		default:
			fatal(
				fmt.Sprintf(
					"create application failed: %v",
					err,
				),
			)
		}
	}

	auditID, auditErr := auth.NewOpaqueToken("aud_")
	if auditErr != nil {
		fatal(auditErr.Error())
	}
	if err := storesqlite.NewStore(db).RecordAuditEvent(ctx, audit.Event{ID: auditID, EventType: "identity.application.operator_provisioned", Status: "success", Severity: "warning", ReasonCode: "operator_cli", Metadata: map[string]any{"client_id": result.Application.ClientID, "internal": result.Application.Internal}, CreatedAt: time.Now().UTC()}); err != nil {
		fatal("provisioning audit failed")
	}
	if *asJSON {
		payload := map[string]any{
			"application": map[string]any{
				"id":            result.Application.ID,
				"name":          result.Application.Name,
				"client_id":     result.Application.ClientID,
				"type":          result.Application.ApplicationType,
				"internal":      result.Application.Internal,
				"redirect_uris": result.Application.RedirectURIs,
				"scopes":        result.Application.Scopes,
				"audiences":     result.Application.Audiences,
			},
			"client_secret": result.ClientSecret,
		}

		_ = json.NewEncoder(os.Stdout).Encode(payload)
		return
	}

	fmt.Printf(
		"created application %s (%s)\n",
		result.Application.ClientID,
		result.Application.ID,
	)

	if result.ClientSecret != "" {
		fmt.Println("")
		fmt.Println(
			"Client secret (shown once):",
		)
		fmt.Println(result.ClientSecret)
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
