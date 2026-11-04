<script lang="ts">
  import { onMount } from 'svelte';
  import Shell from '../lib/Shell.svelte';
  import { api } from '../lib/api';

  let retention = 30;
  let msg = '';
  let error = '';

  onMount(async () => {
    try {
      const s = await api.settings();
      retention = s.file_retention_minutes;
    } catch (e: any) {
      error = e.message;
    }
  });

  async function save() {
    msg = '';
    error = '';
    try {
      await api.saveSettings(retention);
      msg = 'settings saved';
    } catch (e: any) {
      error = e.message;
    }
  }

  async function logout() {
    await api.logout();
    location.href = '/login';
  }
</script>

<Shell active="settings">
  {#if error}
    <div class="alert" role="alert">! {error}</div>
  {/if}
  {#if msg}
    <div class="alert ok" role="status">✓ {msg}</div>
  {/if}

  <section class="panel" style="max-width:640px">
    <h2 class="panel-head">Settings <span class="idx">01</span></h2>

    <label class="field">
      <span class="label">retention</span>
      <div style="display:flex;align-items:center;gap:8px">
        <input type="number" min="1" bind:value={retention} style="max-width:120px" />
        <span class="muted">minutes</span>
      </div>
    </label>

    <p class="hint" style="text-align:left;margin-top:10px">
      job rows are kept · only uploaded file bytes are deleted
    </p>

    <div class="row-actions">
      <button type="button" class="btn btn-primary" style="width:auto" on:click={save}>save settings</button>
      <button type="button" class="btn" on:click={logout}>sign out</button>
    </div>
  </section>
</Shell>
