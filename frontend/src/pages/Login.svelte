<script lang="ts">
  import { api } from '../lib/api';
  let password = '';
  let error = '';
  let busy = false;
  async function submit() {
    busy = true; error = '';
    try {
      await api.login(password);
      location.href = '/';
    } catch (e: any) {
      error = e.message || 'login failed';
    } finally { busy = false; }
  }
</script>

<main>
  <h1>PrintNest</h1>
  <p>Simple, private, self-hosted printing.</p>
  <form on:submit|preventDefault={submit}>
    <label>Admin password
      <input type="password" bind:value={password} autocomplete="current-password" required />
    </label>
    {#if error}<p class="err">{error}</p>{/if}
    <button disabled={busy}>{busy ? 'Signing in...' : 'Sign in'}</button>
  </form>
</main>

<style>
  main { max-width: 22rem; margin: 4rem auto; padding: 0 1rem; font-family: system-ui, sans-serif; }
  label { display: grid; gap: .4rem; }
  input, button { font-size: 1rem; padding: .6rem; }
  button { margin-top: 1rem; width: 100%; }
  .err { color: #b00020; }
</style>
