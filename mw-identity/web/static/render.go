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
	CSRFToken       string
	Error           string
	ReturnTo        string
	RegistrationURL string
	RequiredAuth    string
}

type RegistrationData struct {
	CSRFToken string
	Error     string
	ReturnTo  string
	ClientID  string
	AppName   string
}

type AccountData struct {
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
}

type InfoSection struct {
	Title string
	Body  string
}

type InfoData struct {
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

var (
	loginPT = template.Must(template.New("login-pt").Parse(mustRead("index.html")))
	loginEN = template.Must(template.New("login-en").Parse(mustRead("indexEN.html")))
	account = template.Must(template.New("account").Funcs(template.FuncMap{
		"slice": func(value string, start, end int) string {
			runes := []rune(value)
			if start < 0 || end > len(runes) || start >= end {
				return "MW"
			}
			return string(runes[start:end])
		},
	}).Parse(mustRead("account.html")))
	info         = template.Must(template.New("info").Parse(mustRead("info.html")))
	recovery     = template.Must(template.New("recovery").Parse(mustRead("recovery.html")))
	registration = template.Must(template.New("registration").Parse(mustRead("register.html")))
)

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

func RenderRecovery(w io.Writer) error {
	return recovery.Execute(w, nil)
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
