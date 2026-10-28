package static

import (
	"bytes"
	"strings"
	"testing"
)

func TestCatalogsHaveTheSameKeys(t *testing.T) {
	pt, en := catalogs[LangPT], catalogs[LangEN]
	for key, ptValue := range pt {
		enValue, ok := en[key]
		if !ok {
			t.Errorf("%q is missing in English", key)
			continue
		}
		if strings.TrimSpace(ptValue) == "" || strings.TrimSpace(enValue) == "" {
			t.Errorf("%q is empty in one language", key)
		}
		if strings.Count(ptValue, "%s") != strings.Count(enValue, "%s") {
			t.Errorf("%q has different placeholders: %q / %q", key, ptValue, enValue)
		}
	}
	for key := range en {
		if _, ok := pt[key]; !ok {
			t.Errorf("%q is missing in Portuguese", key)
		}
	}
}

func TestTFallsBackSafely(t *testing.T) {
	if got := T("fr", "account.apps_title"); got != catalogs[LangPT]["account.apps_title"] {
		t.Fatalf("unknown language did not fall back to Portuguese: %q", got)
	}
	if got := T(LangEN, "account.remove_confirm", "WOI"); got != "Disconnect MW sign-in for WOI" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestValueLabelsKnownValuesAndKeepsUnknownOnes(t *testing.T) {
	if got := Value(LangPT, "password"); got != "Senha" {
		t.Fatalf("Value(pt, password) = %q", got)
	}
	if got := Value(LangEN, "totp"); got != "Authenticator app" {
		t.Fatalf("Value(en, totp) = %q", got)
	}
	if got := Value(LangPT, "smartcard"); got != "smartcard" {
		t.Fatalf("unknown value was not kept raw: %q", got)
	}
}

// Every catalog-driven page renders in both languages without leaking a raw
// key, which is what T returns when a template asks for a missing one.
func TestPagesRenderInBothLanguagesWithoutRawKeys(t *testing.T) {
	for _, lang := range []string{LangPT, LangEN} {
		page := Page{Lang: lang, LangSwitchURL: "/x?lang=other"}
		renders := map[string]func(*bytes.Buffer) error{
			"account": func(b *bytes.Buffer) error {
				return RenderAccount(b, AccountData{Page: page, StrongSetupContinue: "/oauth/authorize?client_id=x", Notice: "app_removed"})
			},
			"info": func(b *bytes.Buffer) error {
				return RenderInfo(b, InfoData{Page: page, Kind: "privacy", Title: "T", Lead: "L"})
			},
			"recovery": func(b *bytes.Buffer) error { return RenderRecovery(b, RecoveryData{Page: page}) },
			"register": func(b *bytes.Buffer) error {
				return RenderRegistration(b, RegistrationData{Page: page, AppName: "WOI"})
			},
			"continue": func(b *bytes.Buffer) error { return RenderContinue(b, ContinueData{Lang: lang, Target: "/account"}) },
		}
		for name, render := range renders {
			var output bytes.Buffer
			if err := render(&output); err != nil {
				t.Fatalf("%s/%s: %v", lang, name, err)
			}
			body := output.String()
			if !strings.Contains(body, `lang="`+lang+`"`) {
				t.Errorf("%s/%s: html lang is not %s", lang, name, lang)
			}
			for key := range catalogs[lang] {
				if strings.Contains(body, ">"+key+"<") || strings.Contains(body, `"`+key+`"`) {
					t.Errorf("%s/%s: raw key %q in output", lang, name, key)
				}
			}
		}
	}
}
