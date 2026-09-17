<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Printer, type Device } from '../lib/api';
  let printers: Printer[] = [];
  let devices: Device[] = [];
  let error = '';
  let busy = false;
  let name = '';
  let uri = '';
  let model = '';

  onMount(async () => {
    try { printers = await api.printers(); }
    catch (e: any) { error = e.message; }
  });

  async function scan() {
    busy = true; error = '';
    try { devices = await api.discover(); }
    catch (e: any) { error = e.message; }
    finally { busy = false; }
  }

  async function add() {
    error = '';
    try {
      await api.addPrinter(name, uri, model);
      printers = await api.printers();
      name = uri = model = '';
    } catch (e: any) { error = e.message; }
  }

  function use(d: Device) {
    uri = d.uri;
    model = d.make_model;
    if (!name && d.info) name = d.info.toLowerCase().replace(/[^a-z0-9]+/g, '');
  }
</script>

<main>
  <h1>Printers</h1>
  {#if error}<p class="err">{error}</p>{/if}
  <ul>
    {#each printers as p}
      <li><strong>{p.name}</strong> {p.make_model} - {p.state}
        {#if p.state_reasons?.length} ({p.state_reasons.join(', ')}){/if}
      </li>
    {/each}
  </ul>

  <h2>Discover</h2>
  <button disabled={busy} on:click={scan}>{busy ? 'Scanning...' : 'Scan network'}</button>
  <ul>
    {#each devices as d}
      <li>{d.uri} {d.make_model} {d.info} <button on:click={() => use(d)}>Use</button></li>
    {/each}
  </ul>

  <h2>Add printer</h2>
  <label>Name <input bind:value={name} placeholder="deskjet2700" /></label>
  <label>Device URI <input bind:value={uri} placeholder="ipp://192.0.2.10/ipp/print" /></label>
  <label>Model <input bind:value={model} placeholder="HP DeskJet 2700" /></label>
  <button on:click={add}>Add (driverless everywhere)</button>
  <p><a href="/">Back</a></p>
</main>

<style>
  main { max-width: 44rem; margin: 0 auto; padding: 1rem; font-family: system-ui, sans-serif; }
  label { display: block; margin: .5rem 0; }
  input, button { font-size: 1rem; padding: .5rem; }
  .err { color: #b00020; }
</style>
