<script lang="ts">
  import { onMount } from 'svelte';
  import Shell from '../lib/Shell.svelte';
  import { api, type Printer, type Device } from '../lib/api';
  import { printerStateTone } from '../lib/format';

  let printers: Printer[] = [];
  let devices: Device[] = [];
  let error = '';
  let busy = false;
  let name = '';
  let uri = '';
  let model = '';

  $: chips = [{ label: `${printers.length} printer${printers.length === 1 ? '' : 's'}` }];

  onMount(async () => {
    try {
      printers = await api.printers();
    } catch (e: any) {
      error = e.message;
    }
  });

  async function scan() {
    busy = true;
    error = '';
    try {
      devices = await api.discover();
    } catch (e: any) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function add() {
    error = '';
    try {
      await api.addPrinter(name, uri, model);
      printers = await api.printers();
      name = uri = model = '';
    } catch (e: any) {
      error = e.message;
    }
  }

  function use(d: Device) {
    uri = d.uri;
    model = d.make_model;
    if (!name && d.info) name = d.info.toLowerCase().replace(/[^a-z0-9]+/g, '');
  }
</script>

<Shell active="printers" {chips}>
  {#if error}
    <div class="alert" role="alert">! {error}</div>
  {/if}

  <div class="grid">
    <section class="panel">
      <h2 class="panel-head">Printers <span class="idx">01</span></h2>

      {#if printers.length}
        <div class="list">
          {#each printers as p}
            <div class="list-row">
              <span class="title">{p.name}</span>
              <span class="chip {printerStateTone(p.state)}"><span class="dot"></span>{p.state}</span>
              <span class="sub">{p.make_model || 'driverless'}{p.accepting === false ? ' · not accepting jobs' : ''}</span>
              <span class="sub">{p.device_uri || p.uri || 'no device uri'}</span>
              {#if p.state_reasons?.length}
                <span class="sub">reasons: {p.state_reasons.join(', ')}</span>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty">[ no printers configured ]</div>
      {/if}
    </section>

    <div>
      <section class="panel">
        <h2 class="panel-head">Discover <span class="idx">02</span></h2>
        <button type="button" class="btn" disabled={busy} on:click={scan}>
          {busy ? 'scanning…' : 'scan network'}
        </button>

        {#if devices.length}
          <div class="list" style="margin-top:12px">
            {#each devices as d}
              <div class="list-row device">
                <span class="grow">
                  <span class="title">{d.make_model || d.info || 'device'}</span>
                  <span class="uri">{d.uri}</span>
                </span>
                <button type="button" class="btn" on:click={() => use(d)}>use</button>
              </div>
            {/each}
          </div>
        {:else}
          <p class="hint" style="text-align:left;margin-top:10px">
            mDNS scan · needs avahi + dbus. direct ipp:// add works without it.
          </p>
        {/if}
      </section>

      <section class="panel">
        <h2 class="panel-head">Add printer <span class="idx">03</span></h2>

        <label class="field">
          <span class="label">name</span>
          <input type="text" bind:value={name} placeholder="deskjet2700" />
        </label>
        <label class="field">
          <span class="label">device uri</span>
          <input type="text" bind:value={uri} placeholder="ipp://192.168.1.50/ipp/print" />
        </label>
        <label class="field">
          <span class="label">model</span>
          <input type="text" bind:value={model} placeholder="HP DeskJet 2700" />
        </label>

        <button type="button" class="btn btn-primary" disabled={!name || !uri} on:click={add}>
          add · driverless everywhere
        </button>
        <p class="hint">if driverless fails, install hplip and use its PPD</p>
      </section>
    </div>
  </div>
</Shell>
