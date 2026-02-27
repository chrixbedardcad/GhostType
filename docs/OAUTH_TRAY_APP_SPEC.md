# OAuth Implementation Spec for a Windows Tray App

**Version:** 1.0  
**Date:** 2026-02-26  
**Target:** Desktop tray app (Windows) with secure user authentication + API access

---

## 1) Goal

Implement a secure OAuth-based sign-in flow for a Windows tray app, with:
- user sign-in/sign-out
- persistent session
- refresh token rotation
- secure token storage
- backend-mediated API access (recommended)

This spec assumes modern OAuth 2.1 best practices: **Authorization Code + PKCE**.

---

## 2) Architecture (Recommended)

### Components
1. **Windows Tray App (Public Client)**
   - No embedded client secret.
   - Opens browser for login.
   - Handles redirect back to local loopback URI.

2. **OAuth Provider (IdP)**
   - e.g., Auth0, Okta, Azure AD, Cognito, Google, custom OIDC provider.

3. **Your Backend API (Confidential Client)**
   - Validates user identity/session.
   - Stores provider refresh/access tokens securely (server-side).
   - Calls third-party APIs with server-side credentials.

4. **Model/API Provider**
   - Called from backend (preferred), not directly from desktop app.

### Why this design
- Prevents exposing sensitive API keys in desktop binary.
- Allows account controls, revocation, audit, and policy enforcement.
- Supports multi-provider auth and team/org policy.

---

## 3) OAuth Flow Choice

## Flow: Authorization Code + PKCE (desktop-safe)

1. Tray app generates:
   - `code_verifier` (high entropy random)
   - `code_challenge` (SHA256 + Base64URL)
   - `state` and `nonce`
2. Opens system browser to provider `/authorize`.
3. Provider authenticates user + consent.
4. Redirect to loopback callback (e.g. `http://127.0.0.1:49152/callback`).
5. Tray app exchanges code at provider `/token` with `code_verifier`.
6. Receives tokens (or app session token from your backend, depending on design).
7. Tokens/session stored securely.

---

## 4) Redirect URI Strategy (Desktop)

Use one of these:

1. **Loopback redirect (recommended)**
   - `http://127.0.0.1:{randomPort}/callback`
   - App starts temporary localhost listener.

2. **Custom URI scheme**
   - `myapp://oauth/callback`
   - Requires protocol handler registration.

Loopback is usually easiest and most interoperable.

---

## 5) Security Requirements

Mandatory controls:
- PKCE required (`S256` only)
- Validate `state` on callback
- Validate OIDC `nonce` if using ID token
- Use system browser (not embedded webview)
- Store tokens via **Windows DPAPI / Credential Manager**
- Never log tokens, auth code, or secrets
- TLS only for network calls
- Token expiry checks before each protected request
- Refresh token rotation support
- Handle revocation/logout cleanly

Optional hardening:
- Certificate pinning for backend domain
- Device binding / signed device identity
- Short-lived app session tokens (backend-issued)

---

## 6) Token Strategy

### Best pattern
- Tray app stores only your backend session token (short-lived JWT + refresh session)
- Backend stores OAuth provider tokens and third-party API keys

### Avoid
- Long-lived third-party refresh tokens in desktop app
- Shipping any provider client secret in app
- Direct model-provider key in local config

---

## 7) Backend Contract (Example)

## Auth endpoints
- `GET /auth/start` → returns provider auth URL + state metadata
- `GET /auth/callback` → handles provider callback, creates app session
- `POST /auth/refresh` → rotates app session token
- `POST /auth/logout` → revokes session/tokens
- `GET /auth/me` → returns current user profile + scopes

## API proxy endpoints
- `POST /api/chat`
- `POST /api/tools/...`

Backend attaches provider/API credentials and enforces:
- quotas
- org policy
- audit logging

---

## 8) Windows Tray App Behavior Spec

### Login
1. User clicks **Sign In** from tray menu.
2. App starts local callback listener.
3. App opens browser with authorize URL.
4. On callback success:
   - exchange code/token (or backend session finalize)
   - securely persist session
   - tray status changes to **Signed In**.

### Startup
- App loads secure session from OS vault.
- If expired and refresh possible, refresh silently.
- If refresh fails, mark signed out.

### Logout
- Clear local session from secure store.
- Call backend logout/revoke endpoint.
- Update tray status.

### Failure UX
- Show friendly notification for:
  - network errors
  - auth timeout
  - revoked session
  - invalid callback state

---

## 9) Data Storage Spec

Store locally:
- session token (encrypted)
- refresh session token (encrypted, if used)
- minimal user profile cache (non-sensitive)

Do **not** store:
- provider client secret
- model provider API keys
- raw long-lived OAuth provider refresh token (unless absolutely required)

Recommended Windows storage:
- DPAPI (`ProtectedData`) for encrypted blobs
- Windows Credential Manager for credential-like items

---

## 10) Scopes & Consent

Request minimum scopes only.

Example:
- `openid profile email offline_access`
- optional API scopes only when needed

Support incremental consent if provider supports it.

---

## 11) Threat Model Checklist

Threats to mitigate:
- code interception → PKCE + loopback + state
- token theft from logs → redact all auth payloads
- binary reverse engineering → no embedded secrets
- session replay → short TTL + refresh rotation + backend checks
- local malware access → OS secure storage + minimal token scope

---

## 12) Observability

Log (safe):
- login started/succeeded/failed
- refresh succeeded/failed
- token expiry timestamps (not token values)
- provider error codes

Do not log:
- auth code
- access token
- refresh token
- ID token payload unless sanitized

---

## 13) Testing Plan

### Unit tests
- PKCE generation/verification helper
- state/nonce validator
- token expiry decision logic

### Integration tests
- authorize → callback → token exchange happy path
- refresh path
- logout/revocation path
- invalid state handling
- expired token recovery

### Security tests
- callback replay attempt
- wrong-state callback rejection
- token redaction in logs
- secure store read/write failure handling

---

## 14) Rollout Plan

1. Implement login + callback + secure storage
2. Add refresh + silent sign-in
3. Add backend API proxy integration
4. Add logout/revoke + audit trail
5. Add telemetry dashboards and alerting

---

## 15) Practical Notes for AI/LLM Apps

If your app calls LLM APIs:
- Use OAuth for **user identity** and account/session management.
- Keep LLM/API keys server-side.
- Desktop app should call your backend, never direct with production keys.

---

## 16) Definition of Done

Feature is done when:
- user can sign in/out reliably from tray
- session survives restart securely
- refresh is silent and robust
- no secrets are in app binary/config/logs
- backend mediates all privileged API calls
- audit logs and revocation work

---

## 17) Optional Next Step

If you want, I can generate a second file with a **concrete implementation blueprint** for your exact stack (e.g., Electron + Node, .NET/WPF, or Tauri), including endpoint shapes and code skeletons.