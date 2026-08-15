(function () {
  "use strict";
  var root = document.documentElement;
  var params = new URLSearchParams(location.search);
  var requiredAuth = document.body.dataset.requiredAuth || "";
  var webauthnOnly = requiredAuth === "webauthn";
  var errorBox = document.querySelector("[data-error]");
  var english = root.lang.toLowerCase().indexOf("en") === 0;
  var csrf =
    document.querySelector('meta[name="csrf-token"]').content ||
    params.get("csrf") ||
    "";
  document.querySelectorAll("[data-csrf]").forEach(function (input) {
    input.value = csrf;
  });
  document.querySelectorAll("[data-return-to]").forEach(function (input) {
    input.value = params.get("return_to") || input.value || "";
  });
  var languageLink = document.querySelector("[data-language-link]");
  if (languageLink && params.toString()) {
    languageLink.href += "?" + params.toString();
  }
  function show(name, preserveError) {
    if (webauthnOnly && name !== "step-up" && name !== "success") {
      name = "step-up";
    }
    document.querySelectorAll("[data-screen]").forEach(function (screen) {
      screen.classList.toggle("active", screen.dataset.screen === name);
    });
    if (!preserveError) {
      errorBox.textContent = "";
      errorBox.hidden = true;
    }
    var nextParams = new URLSearchParams(location.search);
    nextParams.set("screen", name);
    history.replaceState(
      {},
      "",
      location.pathname + "?" + nextParams.toString(),
    );
  }
  document.querySelectorAll("[data-show]").forEach(function (button) {
    button.addEventListener("click", function () {
      show(button.dataset.show, false);
    });
  });
  show(
    (webauthnOnly ? "step-up" : params.get("screen")) ||
      (params.get("step_up") === "true" || requiredAuth === "strong"
        ? "step-up"
        : "login"),
    true,
  );

  var appName = params.get("app");
  if (appName) {
    document.querySelector("[data-app-context]").hidden = false;
    document.querySelector("[data-app-name]").textContent = appName;
    document.querySelector("[data-app-monogram]").textContent = appName
      .slice(0, 2)
      .toUpperCase();
    document.querySelector("[data-consent-app]").textContent = appName;
  }

  document
    .querySelector("[data-password-toggle]")
    .addEventListener("click", function (event) {
      var input = event.currentTarget.parentElement.querySelector("input");
      var hidden = input.type === "password";
      input.type = hidden ? "text" : "password";
      event.currentTarget.textContent = hidden
        ? english
          ? "Hide"
          : "Ocultar"
        : english
          ? "Show"
          : "Mostrar";
    });
  document
    .querySelector("[data-code-form] input")
    .addEventListener("input", function (event) {
      event.target.value = event.target.value.replace(/\D/g, "").slice(0, 6);
    });

  function bytes(value) {
    var normalized = value.replace(/-/g, "+").replace(/_/g, "/");
    var raw = atob(normalized);
    return Uint8Array.from(raw, function (char) {
      return char.charCodeAt(0);
    }).buffer;
  }
  function base64(value) {
    var bytesValue = new Uint8Array(value);
    var raw = "";
    bytesValue.forEach(function (byte) {
      raw += String.fromCharCode(byte);
    });
    return btoa(raw).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }
  function message(text) {
    errorBox.textContent = text;
    errorBox.hidden = false;
  }
  async function passkey(optionsPath, verifyPath) {
    try {
      if (!window.PublicKeyCredential)
        throw new Error(
          english
            ? "Passkeys are not supported by this browser."
            : "Passkeys não são compatíveis com este navegador.",
        );
      var optionsResponse = await fetch(optionsPath, {
        method: "POST",
        credentials: "same-origin",
        headers: { Accept: "application/json" },
      });
      if (!optionsResponse.ok)
        throw new Error(
          english
            ? "The confirmation could not be started."
            : "Não foi possível iniciar a confirmação.",
        );
      var payload = await optionsResponse.json();
      var options =
        (payload.options && payload.options.publicKey) ||
        payload.publicKey ||
        payload;
      options.challenge = bytes(options.challenge);
      if (options.allowCredentials)
        options.allowCredentials = options.allowCredentials.map(
          function (item) {
            item.id = bytes(item.id);
            return item;
          },
        );
      var credential = await navigator.credentials.get({ publicKey: options });
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
      };
      var returnTo = params.get("return_to") || "";
      var verifyURL =
        verifyPath +
        (returnTo ? "?return_to=" + encodeURIComponent(returnTo) : "");
      var verify = await fetch(verifyURL, {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-MW-WebAuthn-Ceremony": payload.ceremony_token || "",
        },
        body: JSON.stringify(result),
      });
      if (!verify.ok)
        throw new Error(
          english
            ? "The passkey could not be verified."
            : "A passkey não pôde ser confirmada.",
        );
      var body = await verify.json().catch(function () {
        return {};
      });
      if (body.redirect_to) location.assign(body.redirect_to);
      else show("success");
    } catch (error) {
      if (error.name !== "NotAllowedError")
        message(
          error.message ||
            (english
              ? "This passkey could not be used."
              : "Não foi possível usar esta passkey."),
        );
    }
  }
  document.querySelectorAll("[data-passkey-login]").forEach(function (button) {
    button.addEventListener("click", function () {
      passkey("/api/webauthn/login/options", "/api/webauthn/login/verify");
    });
  });
  document
    .querySelector("[data-recovery-passkey]")
    .addEventListener("click", function () {
      location.assign("/recover");
    });
  document
    .querySelector("[data-code-form]")
    .addEventListener("submit", async function (event) {
      event.preventDefault();
      var email = document.querySelector('[data-password-form] [name="email"]');
      var code = event.currentTarget.querySelector('[name="code"]');
      try {
        if (!email.value.trim()) {
          throw new Error(
            english
              ? "Enter your email before using a TOTP code."
              : "Informe seu e-mail antes de usar o código TOTP.",
          );
        }
        var response = await fetch("/api/totp/login", {
          method: "POST",
          credentials: "same-origin",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            email: email.value,
            code: code.value,
            return_to: params.get("return_to") || "",
          }),
        });
        var body = await response.json().catch(function () {
          return {};
        });
        if (!response.ok) {
          throw new Error(
            english ? "The TOTP code is invalid." : "O código TOTP é inválido.",
          );
        }
        location.assign(body.redirect_to || "/account");
      } catch (error) {
        message(error.message);
      }
    });
  document.querySelector("[data-close]").addEventListener("click", function () {
    if (window.opener) window.close();
    else location.assign(params.get("return_to") || "/");
  });
})();
