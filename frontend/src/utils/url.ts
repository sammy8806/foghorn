// Alert content (generatorURL, annotations, incident links) is fully controlled
// by the remote alert source. Anything that ends up in an href or is handed to
// the system browser has to be validated first: the webview has no navigation
// policy handler, so an unchecked `javascript:` href runs in the app's JS
// context — with the Wails bridge (window.go.*) in reach — and an unchecked
// `HTTPS://…` page would load *inside* the app rather than the browser.

const ALLOWED_PROTOCOLS = ['http:', 'https:'];

/**
 * Returns the URL if it parses and uses http/https, otherwise null.
 * Protocol comparison is on the parsed URL, so it is case-insensitive and not
 * fooled by whitespace, control characters, or embedded credentials.
 */
export function safeExternalURL(raw: string | null | undefined): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (!trimmed) return null;

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return null;
  }
  if (!ALLOWED_PROTOCOLS.includes(parsed.protocol.toLowerCase())) return null;
  return parsed.href;
}

export function isSafeExternalURL(raw: string | null | undefined): boolean {
  return safeExternalURL(raw) !== null;
}
