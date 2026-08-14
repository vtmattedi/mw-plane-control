;(function () {
  'use strict'
  var root = document.documentElement
  var params = new URLSearchParams(location.search)
  var errorBox = document.querySelector('[data-error]')
  var csrf =
    document.querySelector('meta[name="csrf-token"]').content ||
    params.get('csrf') ||
    ''

  document.querySelectorAll('[data-csrf]').forEach(function (input) {
    input.value = csrf
  })
  var storedTheme = localStorage.getItem('mw-identity-theme')
  if (storedTheme) root.dataset.theme = storedTheme
  document
    .querySelector('[data-theme-toggle]')
    .addEventListener('click', function () {
      var dark =
        root.dataset.theme === 'dark' ||
        (!root.dataset.theme &&
          matchMedia('(prefers-color-scheme: dark)').matches)
      root.dataset.theme = dark ? 'light' : 'dark'
      localStorage.setItem('mw-identity-theme', root.dataset.theme)
    })

  function show(name) {
    document.querySelectorAll('[data-screen]').forEach(function (screen) {
      screen.classList.toggle('active', screen.dataset.screen === name)
    })
    errorBox.hidden = true
    history.replaceState(
      {},
      '',
      location.pathname + '?screen=' + encodeURIComponent(name),
    )
  }
  document.querySelectorAll('[data-show]').forEach(function (button) {
    button.addEventListener('click', function () {
      show(button.dataset.show)
    })
  })
  show(
    params.get('screen') ||
      (params.get('step_up') === 'true' ? 'step-up' : 'login'),
  )

  var appName = params.get('app')
  if (appName) {
    document.querySelector('[data-app-context]').hidden = false
    document.querySelector('[data-app-name]').textContent = appName
    document.querySelector('[data-app-monogram]').textContent = appName
      .slice(0, 2)
      .toUpperCase()
    document.querySelector('[data-consent-app]').textContent = appName
  }

  document
    .querySelector('[data-password-toggle]')
    .addEventListener('click', function (event) {
      var input = event.currentTarget.parentElement.querySelector('input')
      var hidden = input.type === 'password'
      input.type = hidden ? 'text' : 'password'
      event.currentTarget.textContent = hidden ? 'Ocultar' : 'Mostrar'
    })
  document
    .querySelector('[data-code-form] input')
    .addEventListener('input', function (event) {
      event.target.value = event.target.value.replace(/\D/g, '').slice(0, 6)
    })

  function bytes(value) {
    var normalized = value.replace(/-/g, '+').replace(/_/g, '/')
    var raw = atob(normalized)
    return Uint8Array.from(raw, function (char) {
      return char.charCodeAt(0)
    }).buffer
  }
  function base64(value) {
    var bytesValue = new Uint8Array(value)
    var raw = ''
    bytesValue.forEach(function (byte) {
      raw += String.fromCharCode(byte)
    })
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }
  function message(text) {
    errorBox.textContent = text
    errorBox.hidden = false
  }
  async function passkey(optionsPath, verifyPath) {
    try {
      if (!window.PublicKeyCredential)
        throw new Error('Passkeys não são compatíveis com este navegador.')
      var optionsResponse = await fetch(optionsPath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: '{}',
      })
      if (!optionsResponse.ok)
        throw new Error('Não foi possível iniciar a confirmação.')
      var payload = await optionsResponse.json()
      var options = payload.publicKey || payload
      options.challenge = bytes(options.challenge)
      if (options.allowCredentials)
        options.allowCredentials = options.allowCredentials.map(
          function (item) {
            item.id = bytes(item.id)
            return item
          },
        )
      var credential = await navigator.credentials.get({ publicKey: options })
      var result = {
        id: credential.id,
        rawId: base64(credential.rawId),
        type: credential.type,
        response: {
          authenticatorData: base64(credential.response.authenticatorData),
          clientDataJSON: base64(credential.response.clientDataJSON),
          signature: base64(credential.response.signature),
          userHandle: credential.response.userHandle
            ? base64(credential.response.userHandle)
            : null,
        },
      }
      var verify = await fetch(verifyPath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify(result),
      })
      if (!verify.ok) throw new Error('A passkey não pôde ser confirmada.')
      var body = await verify.json().catch(function () {
        return {}
      })
      if (body.redirect_to) location.assign(body.redirect_to)
      else show('success')
    } catch (error) {
      if (error.name !== 'NotAllowedError')
        message(error.message || 'Não foi possível usar esta passkey.')
    }
  }
  document.querySelectorAll('[data-passkey-login]').forEach(function (button) {
    button.addEventListener('click', function () {
      passkey('/api/webauthn/login/options', '/api/webauthn/login/verify')
    })
  })
  document
    .querySelector('[data-recovery-passkey]')
    .addEventListener('click', function () {
      passkey('/api/recovery/webauthn/options', '/api/recovery/webauthn/verify')
    })
  document.querySelector('[data-close]').addEventListener('click', function () {
    if (window.opener) window.close()
    else location.assign(params.get('return_to') || '/')
  })
})()
