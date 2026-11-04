<script lang="ts">
  import Shell from '../lib/Shell.svelte';
  import { api } from '../lib/api';

  let password = '';
  let error = '';
  let busy = false;

  async function submit() {
    busy = true;
    error = '';
    try {
      await api.login(password);
      location.href = '/';
    } catch (e: any) {
      error = e.message || 'login failed';
    } finally {
      busy = false;
    }
  }
</script>

<Shell active="login">
  <div class="login">
    <section class="panel">
      <h2 class="panel-head">Login required <span class="idx">auth</span></h2>
      <p class="prompt-line">printnest@local:~$ <b>login</b></p>

      <form on:submit|preventDefault={submit}>
        <label class="field">
          <span class="label">admin password</span>
          <input
            type="password"
            bind:value={password}
            autocomplete="current-password"
            required
          />
        </label>

        {#if error}
          <div class="alert" role="alert" style="margin-top:12px">! {error}</div>
        {/if}

        <button class="btn btn-primary" type="submit" disabled={busy}>
          {busy ? '> authenticating…' : '> SIGN_IN'}
        </button>
      </form>

      <p class="hint">single-user instance · one admin password</p>
    </section>
  </div>
</Shell>
