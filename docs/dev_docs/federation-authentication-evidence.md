# Authentication evidence and combined MFA

Session proof is a versioned fact written only after verification: `pwd_v1`, `webauthn_uv_v1`, or `pwd_totp_v1`. It records the actual authentication event through the existing `auth_time`; token issuance never resets it. The proof is carried unchanged into authorization codes/access tokens. Passkey proof requires successful library verification with required UV. Password+TOTP proof requires both factors within one five-minute browser-bound challenge. Old sessions have empty proof and must reauthenticate for v1 assurance; TOTP-only sessions are revoked by migration. Recovery capabilities grant no authentication proof.

Password verification creates no session. A successful password check starts a random challenge whose hash, browser-CSRF binding hash, user ID, password fingerprint, fixed return destination, expiry and five-attempt budget are persisted. Completion checks the same browser, current user/recovery/deletion state and unchanged password, consumes a non-replayed TOTP and atomically consumes the challenge before issuing a session. No password, OTP or raw bearer challenge is persisted. Attempts survive restart. Concurrent completions cannot issue two sessions. Failure after consumption requires restarting authentication.

The portal uses a password then code flow in both languages. Direct email+TOTP login is removed. Login rate limiting and CSRF apply to both stages. Existing WebAuthn-only BFF/Identity admin mutation checks remain in place. Hardware/browser evidence is a separate release gate.

## Published satisfaction matrix and consumer cutover

| Proof | Achieved ACR | AMR | Standard | Strong | WebAuthn-only |
| --- | --- | --- | --- | --- | --- |
| Password | `urn:mw:auth:standard` | `pwd` | yes | no | no |
| Password + TOTP | `urn:mw:auth:strong` | `pwd`, `otp` | yes | yes | no |
| UV passkey | `urn:mw:webauthn` | `webauthn` | yes | yes | yes |
| Missing, legacy TOTP, recovery | none | none | no | no | no |

`acr_values` accepts one exact published value; unknown values or unsupported lists fail explicitly. Default requirement is Standard, augmented by a retained legacy app Strong gate until reviewed cutover. `max_age=0` creates a new authentication transaction/session before issuing a code. Positive age is checked at authorization and exchange. No age resets when exchanging or delegating. Discovery publishes the three implemented ACRs. New ID tokens use the documented AMR identifiers above, replacing legacy password labels.

Consumers validate signature, exact issuer/audience, nonce/state, expiry/issue time, subject and ACR satisfaction, then freshness. Bind step-up to the original app session/subject and action intent; final destructive confirmation remains local. BFF step-up transactions now persist expected subject and required WebAuthn ACR; wrong subject, weaker/missing ACR and stale proof are rejected, while high-risk routes remain WebAuthn-only.

After owner-reviewed consumer adoption and a disposable end-to-end check, `/app/federation-cutover --client-id <client> --retire-app-strong-reviewed --reviewed` retires that client's old app-wide gate and audits the cutover. It does not change access eligibility unless a separate explicit `--access` argument is supplied. Deprecated columns remain through the rollback window. No protected legacy client is automatically downgraded.
