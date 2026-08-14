# MW Web Admin — Project Plan

## Responsibility

`mw-web-admin` is the shared MattediWorks administrative SPA.

Planned modules:

```text
Overview

Identity
├── Users
├── Applications
├── Assignments / roles
├── Sessions
└── Audit

OAuth
├── Projects
├── Google provider configuration
├── Audit
└── Metrics

Mail
├── Projects
├── Configuration
├── Events / usage
├── Audit
└── Metrics
```

StackPort is intentionally not a module.

## Runtime model

The frontend is built statically:

```text
npm run build
-> dist/
```

The production gateway Nginx image serves `dist/`.

No frontend Node runtime is required.

SSR and React Server Components are not planned because this is an
authenticated internal application with no meaningful SEO requirement.

## Repository initialization

The bootstrap initializer creates only:

```text
mw-web-admin/
```

It intentionally does not create a Vite project or install dependencies.

The preferred frontend suite will be installed manually later.

During Identity/BFF development, a deliberately small existing Admin frontend
can be used as a browser test harness.

## Browser API boundary

Normal browser calls go only to:

```text
https://admin.mattediworks.com/api/*
```

The SPA should not directly call privileged admin APIs at:

```text
identity.mattediworks.com
oauth.mattediworks.com
mailgo.mattediworks.com
```

for normal administration.

Those calls belong to the BFF.

Benefits:

- host-only Admin session;
- no long-lived service tokens in JS;
- simpler CORS;
- consistent CSRF;
- one browser API contract;
- downstream topology hidden from the browser.

## Authentication UX

Normal flow:

```text
Admin SPA
-> /api/auth/login
-> Identity
-> authenticate
-> BFF callback
-> Admin session
```

Actual Identity WebAuthn ceremonies happen at the Identity origin.

The Admin UI can initiate step-up authentication but does not own the
Identity credential ceremony.

## Future frontend structure

Recommended:

```text
src/
├── app/
│   ├── router.tsx
│   ├── shell/
│   └── navigation/
├── auth/
├── modules/
│   ├── identity/
│   ├── oauth/
│   └── mail/
├── components/
│   └── ui/
├── lib/
└── styles/
```

Shared:

- branding;
- shell/sidebar;
- theme;
- tables/dialogs;
- errors/toasts;
- loading states;
- auth state.

Module-local:

- API types;
- pages;
- module components;
- business-specific forms;
- module route definitions.

## Test frontend during backend development

Before the final Admin UI is ready, the backend can use a minimal browser
harness capable of:

- initiating OIDC login;
- showing current Identity;
- calling a protected BFF endpoint;
- listing/creating an Identity application;
- later exercising OAuth/Mail module proxy calls.

That test UI is disposable. Backend milestones must not depend on frontend
polish.
