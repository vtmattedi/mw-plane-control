package httpapi

import (
	"net/http"
	"strings"
	"time"

	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

const languageCookieName = "mw_identity_lang"

// languageFromTag maps a language tag ("en-US", "pt", "pt-BR") to a portal
// language, or "" when the tag is neither English nor Portuguese.
func languageFromTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	switch {
	case tag == "en" || strings.HasPrefix(tag, "en-"):
		return webstatic.LangEN
	case tag == "pt" || strings.HasPrefix(tag, "pt-"):
		return webstatic.LangPT
	default:
		return ""
	}
}

// explicitLanguage is a language the request asks for on purpose: a ?lang=
// switch, or a /login/en or /register/en path. These are remembered.
func explicitLanguage(r *http.Request) string {
	if lang := languageFromTag(r.URL.Query().Get("lang")); lang != "" {
		return lang
	}
	if strings.HasSuffix(r.URL.Path, "/en") {
		return webstatic.LangEN
	}
	return ""
}

func rememberedLanguage(r *http.Request) string {
	cookie, err := r.Cookie(languageCookieName)
	if err != nil {
		return ""
	}
	return languageFromTag(cookie.Value)
}

func browserLanguage(r *http.Request) string {
	tag, _, _ := strings.Cut(r.Header.Get("Accept-Language"), ",")
	tag, _, _ = strings.Cut(tag, ";")
	return languageFromTag(tag)
}

// pageLanguage resolves the portal language: an explicit choice, then the
// remembered one, then the browser's first preference. Portuguese otherwise.
func pageLanguage(r *http.Request) string {
	for _, lang := range []string{explicitLanguage(r), rememberedLanguage(r), browserLanguage(r)} {
		if lang != "" {
			return lang
		}
	}
	return webstatic.LangPT
}

// rememberLanguage stores an explicit choice so later pages, including the
// account page, keep it.
func rememberLanguage(w http.ResponseWriter, r *http.Request, secure bool) {
	lang := explicitLanguage(r)
	if lang == "" || lang == rememberedLanguage(r) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: languageCookieName, Value: lang, Path: "/",
		MaxAge: int((365 * 24 * time.Hour).Seconds()), HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

// languageSwitchURL is the current page in the other language. It keeps the
// query (an app's return_to, a client_id) and drops a /en path suffix.
func languageSwitchURL(r *http.Request, lang string) string {
	other := "en"
	if lang == webstatic.LangEN {
		other = "pt"
	}
	query := r.URL.Query()
	query.Set("lang", other)
	path := strings.TrimSuffix(r.URL.Path, "/en")
	if path == "" {
		path = "/"
	}
	return path + "?" + query.Encode()
}

// page resolves and remembers the language for a catalog-driven page.
func (h *identityAuthHandlers) page(w http.ResponseWriter, r *http.Request) webstatic.Page {
	rememberLanguage(w, r, h.secure)
	lang := pageLanguage(r)
	return webstatic.Page{Lang: lang, LangSwitchURL: languageSwitchURL(r, lang)}
}

// loginPath picks the portal language for a sign-in an application started:
// the OIDC ui_locales parameter, then the remembered choice, then the browser.
func loginPath(r *http.Request) string {
	lang := ""
	if locales := strings.Fields(r.URL.Query().Get("ui_locales")); len(locales) > 0 {
		lang = languageFromTag(locales[0])
	}
	if lang == "" {
		lang = rememberedLanguage(r)
	}
	if lang == "" {
		lang = browserLanguage(r)
	}
	if lang == webstatic.LangEN {
		return "/login/en"
	}
	return "/login"
}
