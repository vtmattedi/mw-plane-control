package static

import (
	"html/template"
	"io"
	"net/http"
	"strings"
)

type LoginData struct {
	CSRFToken string
	Error     string
	ReturnTo  string
}

func AssetHandler() http.Handler {
	contentTypes := map[string]string{
		"portal.css":     "text/css; charset=utf-8",
		"portal.js":      "text/javascript; charset=utf-8",
		"icons.svg":      "image/svg+xml",
		"logo-only.svg":  "image/svg+xml",
		"logo-below.svg": "image/svg+xml",
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
)

func RenderLogin(w io.Writer, english bool, data LoginData) error {
	if english {
		return loginEN.Execute(w, data)
	}
	return loginPT.Execute(w, data)
}

func mustRead(name string) string {
	value, err := Files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(value)
}
