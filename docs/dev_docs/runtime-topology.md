# Runtime and Gateway Plan

## Runtime containers

One Compose project with three runtime containers:

```text
gateway
mw-bff
mw-identity
```

`mw-web-admin` is a build-time project, not a runtime container.

## One published host port

Only gateway publishes a host/loopback port.

Target shape:

```text
127.0.0.1:<host-port> -> gateway:8080
```

BFF and Identity use private Docker ports only.

This keeps StackPort close to:

```text
one Compose stack -> one target port
```

while extending it to permit multiple domains pointing to that target.

## Gateway routing

For:

```text
Host: admin.mattediworks.com
```

route:

```text
/        -> Admin SPA static files
/api/*   -> mw-bff
```

For:

```text
Host: identity.mattediworks.com
```

route:

```text
/* -> mw-identity
```

Gateway responsibilities can include:

- static compression;
- immutable cache headers for fingerprinted assets;
- no-cache for `index.html`;
- request size limits;
- selected security headers;
- proxy timeouts.

Do not put application authorization logic in Nginx.

## Build model

Gateway image eventually uses a multi-stage build:

```text
stage 1:
  Node
  build mw-web-admin -> dist/

stage 2:
  nginx:alpine
  copy dist/
  copy gateway config
```

A frontend-only change rebuilds the gateway image, not the BFF binary.

A BFF-only change rebuilds the BFF image without bundling frontend assets.

## Identity UI

Identity needs browser pages under the Identity origin.

For initial implementation these can be plain static/server HTML served by
the Identity Go service.

They are separate from `mw-web-admin`.

This ensures Identity/WebAuthn ceremonies occur on:

```text
https://identity.mattediworks.com
```

## Docker networking

Plan one private control-plane network.

Only gateway is host-facing.

BFF and Identity are reachable by Docker service DNS names.

When BFF later calls existing MW OAuth/Mail stacks, use an explicit protected
cross-stack network or protected host/internal address rather than publishing
new public admin ports only for the BFF.

The exact cross-Compose networking choice is deferred until service
federation.

## TLS

Outer StackPort/host Nginx terminates TLS.

Inner gateway can use HTTP on loopback/private networking.

The canonical public issuer/origins come from fixed trusted configuration, not
from Docker service names.
