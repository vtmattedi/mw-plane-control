package static

import (
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type LoginData struct {
	RestrictedApplication bool
	CSRFToken             string
	Error                 string
	ReturnTo              string
	RegistrationURL       string
	RequiredAuth          string
}

// Page carries the language every catalog-driven page needs: Lang selects the
// copy, LangSwitchURL is the same page in the other language.
type Page struct {
	Lang          string
	LangSwitchURL string
}

type RegistrationData struct {
	Page
	CSRFToken string
	Error     string
	ReturnTo  string
	ClientID  string
	AppName   string
	LoginURL  string
}

type RecoveryData struct {
	Page
}

type AccountData struct {
	DeletionReauthRequired bool
	PasskeyRequired        bool
	Page
	Email           string
	DisplayName     string
	IsSuperadmin    bool
	CSRFToken       string
	AuthMethod      string
	Username        string
	AccountType     string
	EmailVerifiedAt *time.Time
	Applications    []auth.ConnectedApplication
	// Notice is the outcome of a redirect back to /account, e.g. "app_removed".
	Notice string
	// StrongSetupContinue is where to return once a strong credential exists,
	// set only when an application sent the user here to enroll one.
	StrongSetupContinue string
}

type InfoSection struct {
	Title string
	Body  string
}

type InfoData struct {
	Page
	Kind     string
	Title    string
	Lead     string
	BackURL  string
	Sections []InfoSection
}

func AssetHandler() http.Handler {
	contentTypes := map[string]string{
		"portal.css": "text/css; charset=utf-8",
		"portal.js":  "text/javascript; charset=utf-8",
		"theme.js":   "text/javascript; charset=utf-8",
		"icons.svg":  "image/svg+xml",
		"brand.svg":  "image/svg+xml",
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		contentType, ok := contentTypes[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		value, err := Files.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(value)
	})
}

// pageFuncs is available to every catalog-driven template.
var pageFuncs = template.FuncMap{
	"t":     T,
	"value": Value,
	"slice": func(value string, start, end int) string {
		runes := []rune(value)
		if start < 0 || end > len(runes) || start >= end {
			return "MW"
		}
		return string(runes[start:end])
	},
}

func pageTemplate(name, file string) *template.Template {
	return template.Must(template.New(name).Funcs(pageFuncs).Parse(mustRead(file)))
}

var (
	loginPT      = template.Must(template.New("login-pt").Parse(mustRead("index.html")))
	loginEN      = template.Must(template.New("login-en").Parse(mustRead("indexEN.html")))
	account      = pageTemplate("account", "account.html")
	info         = pageTemplate("info", "info.html")
	recovery     = pageTemplate("recovery", "recovery.html")
	registration = pageTemplate("registration", "register.html")
	continuation = pageTemplate("continue", "continue.html")
)

// ContinueData drives the hand-off page shown after a form POST that
// continues to an application.
type ContinueData struct {
	Lang   string
	Target string
}

func RenderContinue(w io.Writer, data ContinueData) error {
	return continuation.Execute(w, data)
}

func RenderLogin(w io.Writer, english bool, data LoginData) error {
	if english {
		return loginEN.Execute(w, data)
	}
	return loginPT.Execute(w, data)
}

func RenderAccount(w io.Writer, data AccountData) error {
	return account.Execute(w, data)
}

func RenderInfo(w io.Writer, data InfoData) error {
	return info.Execute(w, data)
}

func RenderRecovery(w io.Writer, data RecoveryData) error {
	return recovery.Execute(w, data)
}

func RenderRegistration(w io.Writer, data RegistrationData) error {
	return registration.Execute(w, data)
}

func mustRead(name string) string {
	value, err := Files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(value)
}
