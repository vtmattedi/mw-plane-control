package static

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedPortalBundleIsComplete(t *testing.T) {
	for _, name := range []string{
		"index.html",
		"indexEN.html",
		"portal.css",
		"portal.js",
		"icons.svg",
		"logo-only.svg",
		"logo-below.svg",
	} {
		value, err := Files.ReadFile(name)
		if err != nil {
			t.Fatalf("embedded asset %q: %v", name, err)
		}
		if len(value) == 0 {
			t.Fatalf("embedded asset %q is empty", name)
		}
	}
}

func TestAssetHandlerServesOnlyPublicAssets(t *testing.T) {
	handler := AssetHandler()
	for _, test := range []struct {
		path        string
		status      int
		contentType string
	}{
		{path: "/portal.css", status: 200, contentType: "text/css; charset=utf-8"},
		{path: "/icons.svg", status: 200, contentType: "image/svg+xml"},
		{path: "/index.html", status: 404},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s status = %d; want %d", test.path, recorder.Code, test.status)
		}
		if test.contentType != "" && recorder.Header().Get("Content-Type") != test.contentType {
			t.Fatalf("%s content type = %q", test.path, recorder.Header().Get("Content-Type"))
		}
	}
}

func TestRenderLoginLanguagesAndEscaping(t *testing.T) {
	for _, test := range []struct {
		english bool
		want    string
	}{
		{english: false, want: "Boas-vindas"},
		{english: true, want: "Welcome"},
	} {
		var output bytes.Buffer
		err := RenderLogin(&output, test.english, LoginData{
			CSRFToken: `token"><script>`,
			ReturnTo:  `/oauth/authorize?client=x&scope=y`,
		})
		if err != nil {
			t.Fatal(err)
		}
		body := output.String()
		if !strings.Contains(body, test.want) {
			t.Fatalf("rendered page does not contain %q", test.want)
		}
		if strings.Contains(body, `token"><script>`) {
			t.Fatal("template data was not escaped")
		}
		if !strings.Contains(body, `data-return-to`) {
			t.Fatal("return target input missing")
		}
	}
}
