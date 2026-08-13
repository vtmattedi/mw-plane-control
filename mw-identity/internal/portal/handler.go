package portal

import (
	"html/template"
	"io"
)

type LoginData struct {
	CSRFToken string
	ReturnTo  string
	Error     string
}

type AccountData struct {
	Email        string
	DisplayName  string
	IsSuperadmin bool
	CSRFToken    string
}

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>MW Identity — Sign in</title>
</head>
<body>
  <main>
    <h1>MW Identity</h1>
    <h2>Sign in</h2>
    {{if .Error}}<p role="alert">{{.Error}}</p>{{end}}
    <form method="post" action="/login">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      {{if .ReturnTo}}<input type="hidden" name="return_to" value="{{.ReturnTo}}">{{end}}
      <p>
        <label>
          Email
          <input name="email" type="email" autocomplete="username" required>
        </label>
      </p>
      <p>
        <label>
          Password
          <input name="password" type="password" autocomplete="current-password" required>
        </label>
      </p>
      <button type="submit">Sign in</button>
    </form>
  </main>
</body>
</html>`))

var accountTemplate = template.Must(template.New("account").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>MW Identity</title>
</head>
<body>
  <main>
    <h1>MW Identity</h1>
    <h2>Authenticated</h2>
    <dl>
      <dt>Display name</dt>
      <dd>{{.DisplayName}}</dd>
      <dt>Email</dt>
      <dd>{{.Email}}</dd>
      <dt>Superadmin</dt>
      <dd>{{if .IsSuperadmin}}yes{{else}}no{{end}}</dd>
    </dl>
    <form method="post" action="/logout">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <button type="submit">Sign out</button>
    </form>
  </main>
</body>
</html>`))

func RenderLogin(w io.Writer, data LoginData) error {
	return loginTemplate.Execute(w, data)
}

func RenderAccount(w io.Writer, data AccountData) error {
	return accountTemplate.Execute(w, data)
}
