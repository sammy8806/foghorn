# OIDC device authentication

Foghorn can authenticate an Alertmanager source with the OAuth 2.0 Device
Authorization Grant. This is useful for desktop users who should access
Alertmanager with their own identity, including deployments protected by an
OIDC-aware reverse proxy.

## Foghorn configuration

```yaml
sources:
  - name: production-alertmanager
    type: alertmanager
    url: https://alertmanager.example.com
    auth:
      type: oidc
      flow: device
      issuer_url: https://login.example.com/realms/observability
      client_id: foghorn
      # Add offline_access manually if your provider requires it before issuing
      # a refresh token. Foghorn does not add OAuth scopes automatically.
      scopes: [openid, offline_access]
    poll_interval: 30s
    # The source timeout also covers the interactive login. The normal 10s
    # default is too short for a user to complete SSO or MFA.
    timeout: 5m
```

Foghorn discovers the device authorization and token endpoints from the
issuer, opens the verification page in the system browser, polls for the
token, and sends the access token as `Authorization: Bearer <token>`.

`openid` is the only scope Foghorn itself requires. Add another scope only
when the identity provider requires it for refresh tokens or exposes a required
claim through an optional client scope. `email` and `profile` are not needed
for Alertmanager access. Foghorn never adds `offline_access` automatically;
include it in `auth.scopes` when the provider requires it.

## Persistent login

On supported macOS, Linux, and Windows builds, Foghorn saves the token fields it uses in
the desktop's secure credential store by default: access token, refresh token,
ID token, token type, expiry, and acquisition time. All implementations use
service `de.sammy8806.foghorn.oidc`.

- **macOS:** the item is stored in the login Keychain, is local rather than
  iCloud-synchronizable, and is available after the user first unlocks the Mac
  following startup.
- **Linux:** the item is stored through the freedesktop Secret Service D-Bus
  API. GNOME Keyring, KWallet with Secret Service enabled, KeePassXC with its
  Secret Service integration, or another compatible provider must be running
  in the user's graphical session. Foghorn does not invoke `secret-tool` or put
  token material in command-line arguments.
- **Windows:** the item is stored in Windows Credential Manager for the current
  user.

The credential store lets Foghorn reuse a still-valid token after an app or
source configuration restart. When the access token expires, Foghorn uses the
saved refresh token and immediately stores the rotated response. If a
successful refresh omits a new refresh or ID token, Foghorn preserves the
previous one. Refreshing still requires network access to the identity
provider; "offline" login means the user does not need to repeat browser
authentication.

Saved credentials are isolated by source name, issuer/endpoints, client ID,
sorted scopes, and whether `use_id_token` is enabled. Renaming a source or
changing one of those settings creates a new login identity and leaves the old
credential-store item untouched.

To disable persistence for one source:

```yaml
auth:
  type: oidc
  flow: device
  issuer_url: https://login.example.com/realms/observability
  client_id: foghorn
  scopes: [openid]
  persist_tokens: false
```

If the credential store is locked or temporarily unavailable, authentication
continues in memory and Foghorn retries saving the token on later requests. The
About view identifies the active backend and shows the storage error until
access recovers.

### Forget a login

Open **About → OIDC logins → Forget login** to delete a source's saved item and
clear its in-memory token. This removes Foghorn's local credential only; it does
not revoke the refresh token at the identity provider. The next request for
that source starts a new device login.

The same saved login can be inspected or removed without opening the UI:

```bash
foghorn auth list
foghorn auth clear my-alertmanager
```

## Keycloak client

Create a dedicated public client instead of reusing the confidential client
owned by the reverse proxy. For example, create the client `foghorn` with:

- Client authentication: **Off**
- Authorization: **Off**
- OAuth 2.0 Device Authorization Grant: **On**
- Standard flow: **Off** for a device-only client (leaving it on is not
  required)
- Direct access grants, implicit flow, and service accounts: **Off**
- Require PKCE: **Off**; device authorization does not use the authorization
  code flow

The realm discovery document must publish a
`device_authorization_endpoint`. The client must also be permitted to use the
device grant.

### Group-based authorization

If the reverse proxy authorizes users through a claim such as:

```json
{"groups":["monitoring-user"]}
```

add a Keycloak protocol mapper to the `foghorn` client (or a client scope
assigned to it) with:

- Mapper type: **Group Membership**
- Token claim name: `groups`
- Add to access token: **On**
- Add to ID token: optional
- Full group path: **Off** when the proxy expects `monitoring-user` rather than
  `/monitoring-user`

Ensure the user is a member of the group allowed by the proxy.

Do not select **User Realm Role** merely because the mapper is named `groups`.
That mapper puts realm roles such as `offline_access` into the `groups` claim;
it does not emit group memberships.

If the mapper is part of the client's dedicated or a default client scope, no
extra OAuth scope is needed. If it belongs to an optional client scope, add
that scope name to `auth.scopes`.

## Reverse proxy requirements

The proxy in front of Alertmanager must:

1. Accept a bearer token without forcing the browser-session OIDC flow.
2. Validate the token signature and issuer against the identity provider's
   JWKS.
3. Authorize the required claim, such as membership in `monitoring-user`.

For Envoy Gateway, an OIDC and JWT `SecurityPolicy` needs bearer passthrough in
addition to the JWT header extractor:

```yaml
spec:
  oidc:
    provider:
      issuer: https://login.example.com/realms/observability
    clientID: alertmanager
    clientSecret:
      name: alertmanager-oidc-client-secret
    scopes: [openid, email]
    redirectURL: https://alertmanager.example.com/oauth2/callback
    forwardAccessToken: true
    cookieNames:
      accessToken: alertmanager-access-token
    passThroughAuthHeader: true
  jwt:
    optional: true
    providers:
      - name: keycloak
        issuer: https://login.example.com/realms/observability
        remoteJWKS:
          uri: https://login.example.com/realms/observability/protocol/openid-connect/certs
        extractFrom:
          headers:
            - name: Authorization
              valuePrefix: "Bearer "
          cookies: [alertmanager-access-token]
  authorization:
    defaultAction: Deny
    rules:
      - name: allow-monitoring-users
        action: Allow
        principal:
          jwt:
            provider: keycloak
            claims:
              - name: groups
                valueType: StringArray
                values: [monitoring-user]
```

Without `passThroughAuthHeader: true`, Envoy's OIDC filter starts an
interactive login even when Foghorn supplied a bearer token.

See Envoy Gateway's
[`passThroughAuthHeader` API reference](https://gateway.envoyproxy.io/latest/api/extension_types/#oidc)
for the behavior and version-specific schema.

## Troubleshooting

Run Foghorn with `FOGHORN_HTTP_DEBUG=1`. After a token is acquired, debug mode
logs authorization-relevant claims from both the access and ID tokens. It does
not log the encoded tokens or identity claims such as email or username.

```text
oidc: access token authorization claims={"azp":"foghorn","groups":["monitoring-user"],...}
http: GET https://alertmanager.example.com/api/v2/alerts?... -> 200 (...)
```

| Symptom | Likely cause | What to check |
|---|---|---|
| Device endpoint returns `unauthorized_client` or `invalid_client` | The client cannot use the device grant or is confidential | Enable the device grant, turn client authentication off for the public Foghorn client, and verify `client_id` |
| Token endpoint returns `400` every polling interval before login completes | Usually the normal `authorization_pending` response | Complete the browser verification; Foghorn continues polling until success, expiry, or cancellation |
| Token endpoint returns `200`, then Alertmanager returns `302` | The proxy is starting interactive browser login despite the bearer header | Enable bearer-token bypass; for Envoy Gateway set `oidc.passThroughAuthHeader: true` |
| Alertmanager returns `403` | Bearer passthrough works, but JWT validation or claim authorization failed | Compare the access-token claims in the debug log with the proxy policy |
| `groups` exists only in the ID-token log | The mapper does not add the claim to access tokens | Enable **Add to access token**; Foghorn sends the access token by default |
| `groups` contains realm roles such as `offline_access` | A **User Realm Role** mapper writes roles into a claim named `groups` | Replace it with a **Group Membership** mapper |
| `groups` contains `/monitoring-user` but the proxy expects `monitoring-user` | Keycloak emits full group paths | Turn **Full group path** off or update the proxy rule |
| Access-token `groups` contains the expected value, but the response is still `403` | Another JWT constraint is failing | Check issuer, signature/JWKS, audience restrictions, and the deployed proxy policy |

After changing a mapper or client scope, forget the source's saved OIDC login
from the About view and complete device login again.
