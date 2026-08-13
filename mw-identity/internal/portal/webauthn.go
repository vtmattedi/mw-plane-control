package portal

const IdentityWebAuthnLoginJS = `(() => {
  const button = document.getElementById('passkey-login');
  const status = document.getElementById('passkey-status');
  if (!button || !window.PublicKeyCredential || !navigator.credentials) return;

  const b64ToBytes = (value) => {
    const pad = '='.repeat((4 - value.length % 4) % 4);
    const base64 = (value + pad).replace(/-/g, '+').replace(/_/g, '/');
    const raw = atob(base64);
    return Uint8Array.from(raw, c => c.charCodeAt(0));
  };
  const bytesToB64 = (value) => {
    const bytes = new Uint8Array(value);
    let raw = '';
    for (const byte of bytes) raw += String.fromCharCode(byte);
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
  };
  const prepareRequest = (publicKey) => {
    publicKey.challenge = b64ToBytes(publicKey.challenge);
    if (Array.isArray(publicKey.allowCredentials)) {
      publicKey.allowCredentials = publicKey.allowCredentials.map(item => ({
        ...item,
        id: b64ToBytes(item.id),
      }));
    }
    return publicKey;
  };
  const assertionJSON = (credential) => ({
    id: credential.id,
    rawId: bytesToB64(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment || undefined,
    response: {
      clientDataJSON: bytesToB64(credential.response.clientDataJSON),
      authenticatorData: bytesToB64(credential.response.authenticatorData),
      signature: bytesToB64(credential.response.signature),
      userHandle: credential.response.userHandle
        ? bytesToB64(credential.response.userHandle)
        : null,
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  });

  button.hidden = false;
  button.addEventListener('click', async () => {
    button.disabled = true;
    if (status) status.textContent = 'Waiting for passkey…';
    try {
      const begin = await fetch('/api/webauthn/login/options', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
      });
      if (!begin.ok) throw new Error('Could not start passkey login.');
      const started = await begin.json();
      const credential = await navigator.credentials.get({
        publicKey: prepareRequest(started.options.publicKey),
      });
      if (!credential) throw new Error('Passkey authentication was cancelled.');

      const returnInput = document.querySelector('input[name="return_to"]');
      const returnTo = returnInput ? returnInput.value : '';
      const url = '/api/webauthn/login/verify' +
        (returnTo ? '?return_to=' + encodeURIComponent(returnTo) : '');
      const finish = await fetch(url, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-MW-WebAuthn-Ceremony': started.ceremony_token,
        },
        body: JSON.stringify(assertionJSON(credential)),
      });
      if (!finish.ok) throw new Error('Passkey authentication failed.');
      const result = await finish.json();
      window.location.assign(result.redirect_to || '/account');
    } catch (error) {
      if (status) status.textContent = error?.message || 'Passkey authentication failed.';
      button.disabled = false;
    }
  });
})();`

const IdentityWebAuthnAccountJS = `(() => {
  const root = document.getElementById('passkeys');
  if (!root || !window.PublicKeyCredential || !navigator.credentials) return;

  const csrf = document.querySelector('meta[name="mw-identity-csrf"]')?.content || '';
  const list = document.getElementById('passkey-list');
  const label = document.getElementById('passkey-label');
  const register = document.getElementById('passkey-register');
  const status = document.getElementById('passkey-account-status');

  const b64ToBytes = (value) => {
    const pad = '='.repeat((4 - value.length % 4) % 4);
    const base64 = (value + pad).replace(/-/g, '+').replace(/_/g, '/');
    const raw = atob(base64);
    return Uint8Array.from(raw, c => c.charCodeAt(0));
  };
  const bytesToB64 = (value) => {
    const bytes = new Uint8Array(value);
    let raw = '';
    for (const byte of bytes) raw += String.fromCharCode(byte);
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
  };
  const prepareCreation = (publicKey) => {
    publicKey.challenge = b64ToBytes(publicKey.challenge);
    publicKey.user.id = b64ToBytes(publicKey.user.id);
    if (Array.isArray(publicKey.excludeCredentials)) {
      publicKey.excludeCredentials = publicKey.excludeCredentials.map(item => ({
        ...item,
        id: b64ToBytes(item.id),
      }));
    }
    return publicKey;
  };
  const registrationJSON = (credential) => ({
    id: credential.id,
    rawId: bytesToB64(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment || undefined,
    response: {
      clientDataJSON: bytesToB64(credential.response.clientDataJSON),
      attestationObject: bytesToB64(credential.response.attestationObject),
      transports: credential.response.getTransports
        ? credential.response.getTransports()
        : [],
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  });

  const api = async (url, options = {}) => {
    const headers = new Headers(options.headers || {});
    if (options.method && options.method !== 'GET') headers.set('X-MW-CSRF', csrf);
    const response = await fetch(url, {
      credentials: 'same-origin',
      ...options,
      headers,
    });
    if (!response.ok) throw new Error('Passkey operation failed.');
    return response;
  };

  const load = async () => {
    const response = await api('/api/webauthn/credentials');
    const data = await response.json();
    list.replaceChildren();
    for (const item of data.credentials || []) {
      const row = document.createElement('li');
      const title = document.createElement('span');
      title.textContent = item.label || 'Passkey';
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.textContent = 'Remove';
      remove.addEventListener('click', async () => {
        if (!confirm('Remove this passkey?')) return;
        remove.disabled = true;
        try {
          await api('/api/webauthn/credentials/' + encodeURIComponent(item.credential_id), {
            method: 'DELETE',
          });
          await load();
        } catch (error) {
          if (status) status.textContent = error?.message || 'Could not remove passkey.';
          remove.disabled = false;
        }
      });
      row.append(title, document.createTextNode(' '), remove);
      list.append(row);
    }
    if (!list.children.length) {
      const empty = document.createElement('li');
      empty.textContent = 'No passkeys registered.';
      list.append(empty);
    }
  };

  register.addEventListener('click', async () => {
    register.disabled = true;
    if (status) status.textContent = 'Waiting for authenticator…';
    try {
      const begin = await api('/api/webauthn/register/options', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label: label.value || '' }),
      });
      const started = await begin.json();
      const credential = await navigator.credentials.create({
        publicKey: prepareCreation(started.options.publicKey),
      });
      if (!credential) throw new Error('Passkey registration was cancelled.');
      await api('/api/webauthn/register/verify', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-MW-WebAuthn-Ceremony': started.ceremony_token,
        },
        body: JSON.stringify(registrationJSON(credential)),
      });
      label.value = '';
      if (status) status.textContent = 'Passkey registered.';
      await load();
    } catch (error) {
      if (status) status.textContent = error?.message || 'Could not register passkey.';
    } finally {
      register.disabled = false;
    }
  });

  root.hidden = false;
  load().catch(() => {
    if (status) status.textContent = 'Could not load passkeys.';
  });
})();`
