package portal

const IdentityStrongAccountJS = `(async () => {
  const csrf = document.querySelector('meta[name="mw-identity-csrf"]')?.content || '';
  const root = document.getElementById('totp'); if (!root) return;
  const status = document.getElementById('totp-status'); const secret = document.getElementById('totp-secret');
  const enrollment = document.getElementById('totp-enrollment'); const qrCode = document.getElementById('totp-qr-code');
  const code = document.getElementById('totp-code'); const setup = document.getElementById('totp-setup'); const confirm = document.getElementById('totp-confirm'); const disable = document.getElementById('totp-disable');
  const api = async (path, options={}) => { options.headers = {...(options.headers||{}), 'X-MW-CSRF': csrf}; const r=await fetch(path,options); if(!r.ok){let b={};try{b=await r.json()}catch{};throw new Error(b?.error?.code||b?.error||('HTTP '+r.status));} return r; };
  async function load(){const r=await fetch('/api/strong/status',{cache:'no-store'});if(!r.ok)return;const b=await r.json();root.hidden=false;setup.hidden=b.totp_enabled;disable.hidden=!b.totp_enabled;if(status)status.textContent=b.totp_enabled?'Authenticator fallback enabled.':'Authenticator fallback disabled.';}
  setup?.addEventListener('click',async()=>{setup.disabled=true;try{const r=await api('/api/totp/setup',{method:'POST'});const b=await r.json();secret.textContent=b.secret;qrCode.src=b.qr_code_data_url;enrollment.hidden=false;code.focus();}catch(e){status.textContent=e.message;}finally{setup.disabled=false;}});
  code?.addEventListener('input',()=>{code.value=code.value.replace(/\D/g,'').slice(0,6);});
  confirm?.addEventListener('click',async()=>{try{await api('/api/totp/confirm',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code:code.value})});enrollment.hidden=true;qrCode.removeAttribute('src');secret.textContent='';code.value='';await load();}catch(e){status.textContent=e.message;}});
  disable?.addEventListener('click',async()=>{try{await api('/api/totp/disable',{method:'POST'});await load();}catch(e){status.textContent=e.message;}});
  await load();
})();`

const IdentityTOTPLoginJS = `(() => {
  const form=document.getElementById('totp-login-form'); if(!form)return;
  form.addEventListener('submit',async(e)=>{e.preventDefault();const status=document.getElementById('totp-login-status');try{const r=await fetch('/api/totp/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:form.email.value,code:form.code.value,return_to:form.return_to?.value||''})});const b=await r.json();if(!r.ok)throw new Error(b?.error?.code||'TOTP login failed');location.assign(b.redirect_to||'/account');}catch(err){status.textContent=err.message;}});
})();`

const IdentityRecoveryJS = `(() => {
  const status = document.getElementById('recovery-status');
  const button = document.getElementById('recovery-start');
  const token = location.hash.startsWith('#token=')
    ? decodeURIComponent(location.hash.slice(7))
    : '';
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
        ...item, id: b64ToBytes(item.id),
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
      transports: credential.response.getTransports ? credential.response.getTransports() : [],
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  });
  if (!token) {
    status.textContent = 'Recovery token missing from URL fragment.';
    button.disabled = true;
    return;
  }
  button.addEventListener('click', async () => {
    button.disabled = true;
    try {
      let response = await fetch('/api/recovery/webauthn/options', {
        method: 'POST', credentials: 'same-origin', headers: {'X-MW-Recovery': token},
      });
      let started = await response.json();
      if (!response.ok) throw new Error('Recovery capability rejected');
      const credential = await navigator.credentials.create({
        publicKey: prepareCreation(started.options.publicKey),
      });
      if (!credential) throw new Error('Passkey registration was cancelled.');
      response = await fetch('/api/recovery/webauthn/verify', {
        method: 'POST', credentials: 'same-origin',
        headers: {
          'X-MW-Recovery': token,
          'X-MW-WebAuthn-Ceremony': started.ceremony_token,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(registrationJSON(credential)),
      });
      const result = await response.json();
      if (!response.ok) throw new Error('Recovery verification failed');
      history.replaceState(null, '', location.pathname);
      location.assign(result.redirect_to || '/account');
    } catch (error) {
      status.textContent = error?.message || 'Recovery failed.';
      button.disabled = false;
    }
  });
})();`
