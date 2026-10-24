# MW Plane Control: Stackport onboarding

This runbook uses the current managed-project model documented in Stackport's
`Skills.md` and `Stackport.md`: domains select a Compose service/container port;
applications never publish host ports. It assumes Stackport itself is installed
and that the production encryption keys are already configured.

## 1. Projects and domains

Use separate Stackport projects for Plane Control, MW OAuth, MW Mail, and WOI.
They may share a navigation group, such as `MattediWorks`; groups do not connect
their Docker networks or configure authentication.

For Plane Control, select `docker-compose.stackport.yml`, write the environment
file at `.env`, and use `/_gateway-health` with a 30-second health interval.
Pull/inspect the selected Compose file after configuring the environment, refresh
the detected targets, and select `gateway:8080` for both domain routes.

| Project | Domain | Service | Container port |
| --- | --- | --- | --- |
| Plane Control | admin.mattediworks.com | gateway | 8080 |
| Plane Control | identity.mattediworks.com | gateway | 8080 |
| MW OAuth | oauth.mattediworks.com | gateway | 8282 |
| MW Mail | mailgo.mattediworks.com | gateway | 8082 |
| WOI | woi.mattediworks.com | router | 5995 |

The last three targets reflect the nearby repositories' current Compose/router
definitions. Set each service's `PORT` to its declared target (OAuth 8282, Mail
8082); their example environments currently default to 8080. If an actual deployed
service uses another port, declare that port with `expose` and select it instead.

Point each domain's DNS at the VPS, then enable SSL in Stackport domain settings.
Stackport establishes HTTP/ACME routing, issues certificates, and enables HTTPS.
Both Plane Control domains go through its gateway because the gateway dispatches
by Host. Its current Nginx configuration explicitly names these two domains;
changing hostnames also requires updating gateway config and public URLs.

The nearby OAuth, Mail, and WOI templates still contain retired `SP:AUTO` host
publication. Before deploying those repositories, replace the relevant `ports`
block with `expose` for its container port. There must be no other `ports` block
anywhere in the selected managed Compose file. No host `internalPort` is needed.
Prefer named volumes for database persistence. These other repositories have not
been changed by this onboarding-document update.

## 2. Fresh Identity database: root and admin client

An existing `.env` does not create users or application registrations. If the
database already contains the production root/client, retain their existing data
and credentials and skip creation. These commands intentionally reject duplicates.

Deploy Plane Control so Identity starts. The BFF/gateway may remain unavailable
until its client is registered. On the VPS, list the Identity containers:

```bash
docker ps --filter label=com.docker.compose.service=mw-identity --format 'table {{.ID}}\t{{.Names}}\t{{.Status}}'
```

Choose the container belonging to this project. The following are Bash commands
run over SSH, not commands to paste into Stackport's environment editor:

```bash
IDENTITY_CONTAINER='paste-the-identity-container-id'
ROOT_EMAIL='your-admin-email@example.com'
BFF_CLIENT_ID='copy-your-existing-BFF_OIDC_CLIENT_ID-here'

docker exec -it "$IDENTITY_CONTAINER" /app/create-superadmin \
  --email "$ROOT_EMAIL" --display-name 'MattediWorks Administrator'
```

Enter the root password at the secure terminal prompt. It is bootstrap
authentication for enrolling the first passkey, not the final strong-admin login.

Register the BFF's confidential application with the exact callback and the
complete downstream permission set. Keep this list aligned with
`BFF_DELEGATION_SCOPES` in the Plane Control environment:

```bash
delegated_scopes=(
  oauth.projects.read oauth.projects.write oauth.projects.delete
  oauth.credentials.rotate oauth.policy.read oauth.policy.write
  oauth.audit.read oauth.metrics.read oauth.server.read
  mail.projects.read mail.projects.write mail.projects.delete
  mail.credentials.rotate mail.emails.read mail.usage.read mail.server.read
)
scope_flags=()
for scope in "${delegated_scopes[@]}"; do
  scope_flags+=(--scope "$scope")
done

docker exec -it "$IDENTITY_CONTAINER" /app/create-application \
  --name 'MattediWorks Admin' --client-id "$BFF_CLIENT_ID" \
  --type confidential --internal --auth-policy strong \
  --redirect-uri https://admin.mattediworks.com/api/auth/callback \
  --post-logout-redirect-uri https://admin.mattediworks.com/ \
  --scope openid --scope profile --scope email --scope identity.admin \
  "${scope_flags[@]}" \
  --audience mw-identity-admin --audience mw-oauth --audience mw-mail
```

Save the one-time issued secret in Plane Control's Stackport `.env` as
`BFF_OIDC_CLIENT_SECRET`; `BFF_OIDC_CLIENT_ID` must match the client above. The
secret in a previously prepared environment cannot authenticate a newly generated
registration unless replaced with this issued value. Keep production keyrings
unchanged. Recreate/redeploy Plane Control after saving the environment.

Open `https://identity.mattediworks.com/login` directly, sign in with the bootstrap
password, open `/account`, and register the first passkey. Log out and sign in
again using the passkey. Then open `https://admin.mattediworks.com` and complete
the strong Identity login. The global superadmin has the special admin bootstrap
access; it does not need an explicit assignment to this BFF application.

## 3. MW OAuth and MW Mail resource registrations

In the Plane Control admin UI, open Identity -> Applications. Create two
confidential applications:

| Field | MW OAuth | MW Mail |
| --- | --- | --- |
| Client ID | mw-oauth | mw-mail |
| Audience | mw-oauth | mw-mail |
| Required scope | openid | openid |
| Additional scopes | oauth.* entries listed above | mail.* entries listed above |
| Redirect URI | https://oauth.mattediworks.com/_identity-unused-callback | https://mailgo.mattediworks.com/_identity-unused-callback |

Those callbacks are registry placeholders, not endpoints to implement or navigate
to; the current registry requires a redirect even for resource registrations.
The client ID must equal the delegated audience under the current introspection
contract. Save each independently issued secret in that service's own project.
Do not reuse the BFF secret or share OAuth's secret with Mail.

Set the following in MW OAuth's Stackport `.env`:

```dotenv
PORT=8282
ADMIN_AUTH_MODE=hybrid
MW_IDENTITY_INTERNAL_BASE_URL=https://identity.mattediworks.com
MW_IDENTITY_INTROSPECTION_CLIENT_ID=mw-oauth
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=the-oauth-registration-secret
MW_IDENTITY_AUDIENCE=mw-oauth
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

Set the following in MW Mail's Stackport `.env`:

```dotenv
PORT=8082
ADMIN_AUTH_MODE=hybrid
MW_IDENTITY_INTERNAL_BASE_URL=https://identity.mattediworks.com
MW_IDENTITY_INTROSPECTION_CLIENT_ID=mw-mail
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=the-mail-registration-secret
MW_IDENTITY_AUDIENCE=mw-mail
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

Despite the variable's name, use the public HTTPS Identity URL for this topology.
Separate Stackport projects do not automatically share the private Plane Control
network, and Identity itself is not a routed service on `stackport-proxy`.
`http://mw-identity:7100` is valid only when an explicit shared transport network
has been configured. Preserve the services' normal Host/Origin policies and their
other production keys/provider configuration.

In Plane Control's `.env`, confirm:

```dotenv
BFF_OAUTH_BASE_URL=https://oauth.mattediworks.com
BFF_OAUTH_HOST=
BFF_OAUTH_AUDIENCE=mw-oauth
BFF_MAIL_BASE_URL=https://mailgo.mattediworks.com
BFF_MAIL_HOST=
BFF_MAIL_AUDIENCE=mw-mail
```

Redeploy the resource services and log into Plane Control again to acquire the
current scope set. Verify OAuth -> Projects and Mail -> Projects/usage work.
Keep hybrid mode during migration; move each service to `federated` after these
paths and fresh WebAuthn-sensitive operations work. Resource applications do not
need separate user assignments for BFF token exchange in the current code.

## 3b. MW MQTT admin resource registration (optional)

`mqtt-admin` lives in the MQTT broker's Stackport project (see that repository's
`admin/README.md`) and is a downstream service like OAuth and Mail. The BFF
facade is disabled until `BFF_MQTT_BASE_URL` is set, so deploying this code changes
nothing for OAuth or Mail.

1. In Identity -> Applications create a confidential application:

   | Field | MW MQTT |
   | --- | --- |
   | Client ID | mw-mqtt |
   | Audience | mw-mqtt |
   | Required scope | openid |
   | Additional scopes | mqtt.clients.read mqtt.clients.write mqtt.clients.delete mqtt.credentials.rotate mqtt.roles.read mqtt.roles.write mqtt.roles.delete mqtt.server.read |
   | Redirect URI | https://admin-mqtt.mattediworks.com/_identity-unused-callback |

   Save the issued secret as `MW_IDENTITY_INTROSPECTION_CLIENT_SECRET` in the MQTT
   project's `secrets/admin-api.env`, not here.
   That file must also set `ADMIN_AUTH_MODE=federated`: the service defaults to its
   standalone `token` mode and refuses to start without tokens.
2. Edit the BFF's own Identity application: add audience `mw-mqtt` and the eight
   `mqtt.*` scopes to its allowed audiences and scopes. `mqtt.roles.write` and
   `mqtt.roles.delete` are needed only for editing roles; without them the console can still
   list clients and roles, and role changes fail with an authorization error.
3. In this project's `.env` append the eight `mqtt.*` scopes to
   `BFF_DELEGATION_SCOPES` (keep the OAuth and Mail scopes) and set:

   ```dotenv
   BFF_MQTT_BASE_URL=https://admin-mqtt.mattediworks.com
   BFF_MQTT_HOST=
   BFF_MQTT_AUDIENCE=mw-mqtt
   BFF_MQTT_HTTP_TIMEOUT_SECONDS=5
   ```
4. Redeploy Plane Control and sign in again so the session acquires the new scopes.

Facade routes (browser path `/api/mqtt/...`, forwarded to `/admin/api/...`):

| Route | Scope | Step-up |
| --- | --- | --- |
| GET /server, GET /stats | mqtt.server.read | no |
| GET /roles, GET /roles/{name} | mqtt.roles.read | no |
| POST /roles | mqtt.roles.write | yes |
| DELETE /roles/{name} | mqtt.roles.delete | yes |
| POST /roles/{name}/acls, POST /roles/{name}/acls/remove | mqtt.roles.write | yes |
| PUT, DELETE /clients/{username}/roles/{role} | mqtt.roles.write | yes |
| GET /docs, GET /docs/{version} | mqtt.server.read | no |
| GET /clients, GET /clients/{username} | mqtt.clients.read | no |
| POST /clients | mqtt.clients.write | yes (returns a generated password once) |
| POST /clients/{username}/disable | mqtt.clients.write | no (fast kill switch) |
| POST /clients/{username}/enable | mqtt.clients.write | yes |
| POST /clients/{username}/password | mqtt.credentials.rotate | yes |
| DELETE /clients/{username} | mqtt.clients.delete | yes |

A broker outage reaches the browser as `503 {"error":"broker_unavailable"}`; every
other downstream failure is a generic 502.

Admin pages (the sidebar shows an `MQTT` group once the build includes them; the pages
work only when the facade above is enabled):

- `/mqtt/clients`: list clients, create, disable, re-enable, generate a new password and
  delete. Passwords are always generated by the service and shown once in a dialog; the
  browser never sends or stores one. Clients whose roles are not assignable (for example
  `mqtt-admin`) are shown as protected and have no actions. An operation that needs
  fresh WebAuthn is saved in `sessionStorage` (no secrets), the browser goes through the
  Identity step-up, and the operation runs once when it returns.
- `/mqtt/roles`: access roles. List them with their client counts and whether they are
  reserved or assignable; create a role from a template (blank, trusted, read-only, per-client
  isolation with `%u`, broker monitoring) or from scratch; open a role to see its rules and
  the clients holding it, add and remove rules, and delete it. Deleting or editing a role that
  clients hold says how many clients are affected first. The console refuses what the service
  would refuse (the `$CONTROL` channel, publishing into `$` topics, malformed topic filters)
  before sending anything. Every change needs fresh WebAuthn and resumes after the step-up
  like the other operations. The clients page has a per-client **Roles** dialog to give or
  take roles.
- `/mqtt/docs` and `/mqtt/docs/{version}`: the service documentation (connection guide, access
  model, API reference, troubleshooting), served in the language of the interface with the
  same layout as the OAuth and Mail documentation tabs.
- `/mqtt/server`: broker health, version, uptime, connections, traffic, load averages,
  memory, and the read-only roles with their ACLs. It refreshes every 10 seconds and
  flags data that has not been updated for a minute.

## 4. WOI: human login application

In Identity -> Applications create:

- Name: Working On It; client ID: `woi-production`; type: confidential.
- Scopes: `openid profile email`; authentication policy: `standard`, or `strong`
  if this deployment requires passkey/TOTP authentication.
- Exact callback: `https://woi.mattediworks.com/api/auth/mw/callback`.
- Post-logout callback: `https://woi.mattediworks.com/`.
- Account-deletion callback: `https://woi.mattediworks.com/api/auth/mw/lifecycle`.
- Enable public signup only if WOI should accept self-registering public users.
  Stand-alone sign-up at `https://identity.mattediworks.com/register` is a separate
  switch, `IDENTITY_PUBLIC_SIGNUP_ENABLED` (default `true`); set it to `false` in
  the Identity environment to close it.
- Do not grant `identity.admin`, `mw-identity-admin`, or administrative downstream
  scopes to an ordinary WOI login client.

WOI's Compose file reads `backend/.env` for API secrets. Write that path in the
WOI Stackport project, retaining its existing database/JWT/email settings:

```dotenv
APP_URL=https://woi.mattediworks.com
CORS_ORIGINS=https://woi.mattediworks.com
MW_IDENTITY_ISSUER=https://identity.mattediworks.com
MW_IDENTITY_CLIENT_ID=woi-production
MW_IDENTITY_CLIENT_SECRET=the-woi-registration-secret
MW_IDENTITY_REDIRECT_URI=https://woi.mattediworks.com/api/auth/mw/callback
ALLOW_LEGACY_AUTH=false
```

WOI also interpolates its root `.env` into build/runtime legacy-auth flags. Set
`ALLOW_LEGACY_AUTH=false` there too and rebuild/redeploy. Keep Identity credentials
only in `backend/.env`, not frontend build variables.

In Identity -> Assignments, explicitly assign the intended existing Identity users
to WOI. Assign the root as `user` if it should access WOI too; the root's admin
bootstrap exception does not grant implicit access to WOI. This assignment controls
admission, not WOI's own business-admin role.

For existing WOI accounts, retain projects/roles through the documented verified
link using `(identity_issuer, identity_subject)`. Never automatically link by
matching email. WOI owns this local account mapping and its admin role.

## 5. WOI: Mail project and optional Google OAuth project

In Mail -> Projects, create a WOI sending project and configure its allowed request
origin/domain for `https://woi.mattediworks.com` using the service's request-policy
format. Save the project's API key in WOI's `backend/.env`:

```dotenv
EMAIL_PROVIDER=mw
MW_EMAIL_API_URL=https://mailgo.mattediworks.com
MW_EMAIL_API_KEY=the-woi-mail-project-api-key
MW_EMAIL_ORIGIN=https://woi.mattediworks.com
EMAIL_FROM_NAME=Working On It
EMAIL_FROM_ADDRESS=your-verified-sender-address
```

MW Mail itself keeps its Resend provider key/webhook configuration and a verified
sending domain. The project API key is distinct from Mail's Identity introspection
secret. Test a real WOI notification after redeploying.

WOI's current login integration uses MW Identity directly. No MW OAuth project is
required for that login. OAuth -> Projects is for applications consuming the MW
OAuth Google/API-key callback flow; add a WOI project there only when that flow
is actually integrated. Google credentials configured in MW OAuth are provider
credentials, not Identity users or the WOI OIDC client secret.

## Sources

Current Plane Control Compose/gateway/auth/delegation code; Stackport `Skills.md`
and `Stackport.md`; nearby `working-on-it/MW_IDENTITY.md`, `backend/.env.example`,
and router config; nearby OAuth/Mail configuration and Compose definitions.
