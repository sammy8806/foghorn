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
      scopes: [openid]
    poll_interval: 30s
    # The source timeout also covers the interactive login. The normal 10s
    # default is too short for a user to complete SSO or MFA.
    timeout: 5m
```

Foghorn discovers the device authorization and token endpoints from the
issuer, opens the verification page in the system browser, polls for the
token, and sends the access token as `Authorization: Bearer <token>`.

`openid` is the only scope Foghorn itself requires. Add another scope only
when the identity provider exposes a required claim through an optional client
scope. `email` and `profile` are not needed for Alertmanager access.

Foghorn refreshes an expired access token while it remains running. Tokens are
currently stored only in memory, so restarting Foghorn or reloading its source
configuration requires another device login. Requesting `offline_access` does
not make login persistent until refresh-token storage is implemented.

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

After changing a mapper or client scope, obtain a new token by restarting
Foghorn and completing device login again.
