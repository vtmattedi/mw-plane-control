# MW Identity — Manual administrator recovery runbook

Manual recovery is the catastrophic trust anchor from Step 12. It is operator-controlled and does not use email recovery.

## Preconditions

- verify the administrator identity out of band according to operator policy;
- use a secure terminal on the control-plane host;
- ensure the Identity database and WebAuthn credential keyring are healthy;
- do not paste the raw recovery capability into tickets, chat, logs, or shell history beyond what is operationally necessary.

## Start recovery

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --email admin@example.com
```

Starting recovery:

- marks the user recovery-pending;
- revokes Identity sessions, authorization codes, access/delegated tokens;
- generates a short-lived one-time recovery capability;
- stores only its hash;
- prints the raw capability once.

Open the printed recovery URL in the intended trusted browser. The capability is carried in the URL fragment so it is not automatically sent in the HTTP request target.

Register the replacement passkey. Successful recovery removes the old TOTP fallback and replaces the old passkey trust set according to Step 12 behavior. Then perform a fresh normal WebAuthn login before privileged work.

## Cancel an accidental recovery

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --cancel `
  --email admin@example.com
```

Cancellation clears recovery-pending and invalidates the capability, but it does not resurrect sessions that were already revoked.

## Verification

After completion:

1. old BFF/Identity sessions no longer work;
2. the old recovery capability is rejected;
3. old passkeys are not accepted where replacement revoked them;
4. TOTP must be re-enrolled if fallback is desired;
5. fresh WebAuthn authentication obtains high assurance;
6. relevant Identity audit events are present without the raw recovery token.
