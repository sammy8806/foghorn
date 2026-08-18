<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { ForgetOIDCLogin, GetAbout, GetOIDCSessions } from '../../wailsjs/go/main/App';
  import type { main, provider } from '../../wailsjs/go/models';
  import logo from '../assets/images/logo-universal.png';

  const dispatch = createEventDispatcher<{ back: void }>();

  let info: main.AboutInfo = {
    name: 'Foghorn',
    version: '',
    description: '',
    repoURL: '',
    copyright: '',
  };
  let sessions: provider.OIDCSessionInfo[] = [];
  let sessionsLoading = true;
  let confirmSource = '';
  let forgettingSource = '';
  let sessionMessage = '';
  let sessionError = '';

  async function loadSessions() {
    sessionsLoading = true;
    try {
      sessions = await GetOIDCSessions();
      sessionError = '';
    } catch (e) {
      console.warn('GetOIDCSessions failed', e);
      sessionError = 'Could not load OIDC login status.';
    } finally {
      sessionsLoading = false;
    }
  }

  async function forgetLogin(source: string) {
    if (confirmSource !== source) {
      confirmSource = source;
      sessionMessage = '';
      sessionError = '';
      return;
    }
    forgettingSource = source;
    try {
      await ForgetOIDCLogin(source);
      confirmSource = '';
      sessionMessage = `Forgot the local OIDC login for ${source}. Foghorn will ask you to sign in again when it next needs credentials.`;
      await loadSessions();
    } catch (e) {
      console.warn('ForgetOIDCLogin failed', e);
      sessionError = `Could not remove the saved login for ${source}: ${String(e)}`;
    } finally {
      forgettingSource = '';
    }
  }

  function sessionStatus(session: provider.OIDCSessionInfo): string {
    if (session.storageError) return 'Keychain unavailable — using memory-only credentials';
    if (session.saved) return 'Saved in macOS Keychain';
    if (session.active) return 'Active in memory only';
    if (!session.persistenceEnabled) return 'Persistent storage disabled';
    return 'No saved login';
  }

  onMount(async () => {
    try {
      info = await GetAbout();
    } catch (e) {
      // Non-Wails (browser dev) or bridge not ready: keep fallback defaults.
      console.warn('GetAbout failed', e);
    }
    await loadSessions();
  });
</script>

<section class="about">
  <header>
    <button class="back" type="button" on:click={() => dispatch('back')}>← Back</button>
  </header>

  <div class="body">
    <div class="identity">
      <img class="logo" src={logo} alt="Foghorn logo" width="72" height="72" />
      <h1>{info.name}{#if info.version} <span class="version">v{info.version}</span>{/if}</h1>
      {#if info.description}<p class="description">{info.description}</p>{/if}
      {#if info.repoURL}
        <p class="links"><a href={info.repoURL}>GitHub repository</a></p>
      {/if}
      {#if info.copyright}<p class="copyright">{info.copyright}</p>{/if}
    </div>

    {#if sessionsLoading || sessions.length > 0 || sessionError}
      <section class="sessions" aria-labelledby="oidc-sessions-heading">
        <div class="sessions-heading">
          <div>
            <h2 id="oidc-sessions-heading">OIDC logins</h2>
            <p>Tokens stay local. Forgetting a login removes its in-memory token and macOS Keychain entry.</p>
          </div>
        </div>

        {#if sessionsLoading}
          <p class="session-empty">Loading login status…</p>
        {:else}
          {#each sessions as session (session.source)}
            <div class="session-row">
              <div class="session-details">
                <strong>{session.source}</strong>
                <span class:error-status={!!session.storageError}>{sessionStatus(session)}</span>
              </div>
              <div class="session-actions">
                {#if confirmSource === session.source}
                  <button class="cancel" type="button" on:click={() => (confirmSource = '')} disabled={forgettingSource === session.source}>Cancel</button>
                {/if}
                <button
                  class:danger={confirmSource === session.source}
                  type="button"
                  disabled={forgettingSource === session.source || (!session.active && !session.saved && !session.storageError)}
                  on:click={() => forgetLogin(session.source)}
                >
                  {#if forgettingSource === session.source}
                    Forgetting…
                  {:else if confirmSource === session.source}
                    Confirm forget
                  {:else}
                    Forget login
                  {/if}
                </button>
              </div>
            </div>
          {/each}
        {/if}
        {#if sessionMessage}<p class="session-message" role="status">{sessionMessage}</p>{/if}
        {#if sessionError}<p class="session-error" role="alert">{sessionError}</p>{/if}
      </section>
    {/if}
  </div>
</section>

<style>
  .about {
    height: 100%;
    display: flex;
    flex-direction: column;
    color: #e2e8f0;
  }

  header {
    padding: 8px 12px;
    border-bottom: 1px solid #1e293b;
  }

  .back {
    background: transparent;
    border: none;
    color: #93c5fd;
    cursor: pointer;
    font-size: calc(13px * var(--font-scale, 1));
    padding: 0;
  }

  .back:hover {
    text-decoration: underline;
  }

  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 20px;
    padding: 22px 16px;
    overflow-y: auto;
  }

  .identity {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 8px;
  }

  .logo {
    border-radius: 12px;
  }

  h1 {
    font-size: calc(18px * var(--font-scale, 1));
    margin: 4px 0 0;
    font-weight: 600;
  }

  .version {
    color: #94a3b8;
    font-weight: 400;
    font-size: calc(14px * var(--font-scale, 1));
  }

  .description {
    margin: 0;
    color: #cbd5e1;
    max-width: 260px;
  }

  .links {
    margin: 4px 0 0;
  }

  .links a {
    color: #93c5fd;
  }

  .copyright {
    margin: 8px 0 0;
    color: #64748b;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .sessions {
    width: min(100%, 560px);
    border: 1px solid #334155;
    border-radius: 10px;
    background: #111c30;
    text-align: left;
    overflow: hidden;
  }

  .sessions-heading {
    padding: 12px 14px;
    border-bottom: 1px solid #27364d;
  }

  h2 {
    margin: 0;
    font-size: calc(14px * var(--font-scale, 1));
    font-weight: 600;
  }

  .sessions-heading p {
    margin: 3px 0 0;
    color: #94a3b8;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .session-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 14px;
    border-bottom: 1px solid #27364d;
  }

  .session-details {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .session-details strong {
    overflow-wrap: anywhere;
  }

  .session-details span,
  .session-empty {
    color: #94a3b8;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .session-details .error-status,
  .session-error {
    color: #fca5a5;
  }

  .session-actions {
    flex: 0 0 auto;
    display: flex;
    gap: 6px;
  }

  .session-actions button {
    border: 1px solid #475569;
    border-radius: 6px;
    padding: 5px 8px;
    color: #dbeafe;
    background: #1e293b;
    cursor: pointer;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .session-actions button:hover:not(:disabled) {
    background: #334155;
  }

  .session-actions button.danger {
    border-color: #b91c1c;
    color: #fecaca;
    background: #7f1d1d;
  }

  .session-actions button.cancel {
    background: transparent;
  }

  .session-actions button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .session-empty,
  .session-message,
  .session-error {
    margin: 0;
    padding: 10px 14px;
  }

  .session-message {
    color: #86efac;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .session-error {
    font-size: calc(12px * var(--font-scale, 1));
  }
</style>
