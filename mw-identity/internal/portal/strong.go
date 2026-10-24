package portal

const IdentityStrongAccountJS = `(async () => {
  const csrf = document.querySelector('meta[name="mw-identity-csrf"]')?.content || '';
  const root = document.getElementById('totp');
  const status = document.getElementById('totp-status'); const secret = document.getElementById('totp-secret');
  const enrollment = document.getElementById('totp-enrollment'); const qrCode = document.getElementById('totp-qr-code');
  const code = document.getElementById('totp-code'); const setup = document.getElementById('totp-setup'); const confirm = document.getElementById('totp-confirm'); const disable = document.getElementById('totp-disable');
  const strongStatus = document.getElementById('strong-auth-status');
  const pt = (document.documentElement.lang || '').toLowerCase().startsWith('pt');
  const tr = (en, ptText) => (pt ? ptText : en);
  const api = async (path, options={}) => { options.headers = {...(options.headers||{}), 'X-MW-CSRF': csrf}; const r=await fetch(path,options); if(!r.ok){let b={};try{b=await r.json()}catch{};throw new Error(b?.error?.code||b?.error||('HTTP '+r.status));} return r; };
  function relativeTime(unixSeconds){
    const diff = Math.max(0, Math.floor(Date.now()/1000) - unixSeconds);
    if (diff < 60) return tr('just now', 'agora mesmo');
    if (diff < 3600) return tr(Math.floor(diff/60) + ' min ago', 'há ' + Math.floor(diff/60) + ' min');
    if (diff < 86400) return tr(Math.floor(diff/3600) + ' h ago', 'há ' + Math.floor(diff/3600) + ' h');
    return tr(Math.floor(diff/86400) + ' d ago', 'há ' + Math.floor(diff/86400) + ' d');
  }
  async function load(){
    const r=await fetch('/api/strong/status',{cache:'no-store'});
    if(!r.ok)return;
    const b=await r.json();
    if (strongStatus) {
      strongStatus.classList.toggle('weak', !b.recent_webauthn);
      strongStatus.textContent = b.recent_webauthn
        ? tr('Strong sign-in confirmed ', 'Login forte confirmado ') + relativeTime(b.auth_time) + '.'
        : tr('No recent strong sign-in (session started ' + relativeTime(b.auth_time) + '). Confirm it’s you before sensitive actions.', 'Sem login forte recente (sessão iniciada ' + relativeTime(b.auth_time) + '). Confirme que é você antes de ações sensíveis.');
    }
    if (!root) return;
    root.hidden=false;setup.hidden=b.totp_enabled;disable.hidden=!b.totp_enabled;if(status)status.textContent=b.totp_enabled?tr('Authenticator app on.','App autenticador ativado.'):tr('Authenticator app off.','App autenticador desativado.');
  }
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
  const pt = (document.documentElement.lang || '').toLowerCase().startsWith('pt');
  const tr = (en, ptText) => (pt ? ptText : en);
  // The token lives in the fragment; keep it when switching language.
  const languageLink = document.querySelector('.language-link');
  if (languageLink && location.hash) languageLink.href += location.hash;
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
    status.textContent = tr('This recovery link is missing its token.', 'Este link de recuperação está sem o token.');
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
      if (!response.ok) throw new Error(tr('This recovery link was rejected. Ask the operator for a new one.', 'Este link de recuperação foi recusado. Peça um novo ao operador.'));
      const credential = await navigator.credentials.create({
        publicKey: prepareCreation(started.options.publicKey),
      });
      if (!credential) throw new Error(tr('Passkey registration was cancelled.', 'Cadastro da passkey cancelado.'));
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
      if (!response.ok) throw new Error(tr('The recovery could not be verified.', 'Não deu para validar a recuperação.'));
      history.replaceState(null, '', location.pathname);
      location.assign(result.redirect_to || '/account');
    } catch (error) {
      status.textContent = error?.message || tr('Recovery failed.', 'A recuperação falhou.');
      button.disabled = false;
    }
  });
})();`
