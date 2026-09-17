<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  let retention = 30;
  let msg = '';
  let error = '';
  onMount(async () => {
    try {
      const s = await api.settings();
      retention = s.file_retention_minutes;
    } catch (e: any) { error = e.message; }
  });
  async function save() {
    msg = error = '';
    try {
      await api.saveSettings(retention);
      msg = 'Saved.';
    } catch (e: any) { error = e.message; }
  }
  async function logout() {
    await api.logout();
    location.href = '/login';
  }
</script>

<main>
  <h1>Settings</h1>
  {#if error}<p class="err">{error}</p>{/if}
  {#if msg}<p>{msg}</p>{/if}
  <label>Delete uploaded files after (minutes)
    <input type="number" min="1" bind:value={retention} />
  </label>
  <p>Job rows are kept. Only file bytes are deleted.</p>
  <button on:click={save}>Save</button>
  <button on:click={logout}>Sign out</button>
  <p><a href="/">Back</a></p>
</main>

<style>
  main { max-width: 32rem; margin: 0 auto; padding: 1rem; font-family: system-ui, sans-serif; }
  input, button { font-size: 1rem; padding: .5rem; margin: .3rem 0; }
  .err { color: #b00020; }
</style>
