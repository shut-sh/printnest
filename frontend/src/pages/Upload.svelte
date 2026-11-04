<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import Shell from '../lib/Shell.svelte';
  import JobSummary from '../lib/JobSummary.svelte';
  import { api, type Printer, type Caps, type Job } from '../lib/api';
  import { previewURL, revokeURL, isPDF } from '../lib/preview';
  import {
    fileExt,
    formatBytes,
    isTerminalStatus,
    mediaLabel,
    type Tone
  } from '../lib/format';

  const ACCEPT = 'application/pdf,image/jpeg,image/png,image/webp,image/heic,image/heif';
  const NAME_OK = /\.(pdf|jpe?g|png|webp|heic|heif)$/i;
  const ALLOWED_MIME = new Set([
    'application/pdf',
    'image/jpeg',
    'image/png',
    'image/webp',
    'image/heic',
    'image/heif'
  ]);
  const MAX_BYTES = 50 * 1024 * 1024;

  let printers: Printer[] = [];
  let caps: Caps | null = null;
  let file: File | null = null;
  let url: string | null = null;
  let job: Job | null = null;
  let error = '';
  let busy = false;
  let dragging = false;
  let cupsOk = false;
  let inputEl: HTMLInputElement;

  let printer = '';
  let copies = 1;
  let pageRanges = '';
  let color = 'color';
  let orientation = 'portrait';
  let media = 'iso_a4_210x297mm';
  let scaling = 'fit';

  $: scalingOptions = supported(['none', 'fit', 'fill'], caps?.scaling_supported);
  $: colorOptions = supported(['color', 'monochrome'], caps?.color_mode_supported);
  $: orientationOptions = orientations(caps?.orientation_supported);
  $: mediaOptions = caps?.media_supported?.length ? caps.media_supported : ['iso_a4_210x297mm'];
  $: canPrint = !!file && !!printer && !busy;
  $: chips = [
    { label: cupsOk ? 'cups online' : 'cups offline', tone: (cupsOk ? 'ok' : 'bad') as Tone },
    { label: printer || 'no printer' },
    { label: mediaLabel(media) }
  ];

  function supported(allowed: string[], list?: string[]): string[] {
    if (!list?.length) return allowed;
    const set = new Set(list.map((s) => s.toLowerCase()));
    const out = allowed.filter((a) => set.has(a));
    return out.length ? out : allowed;
  }

  function orientations(list?: number[]) {
    const all = [
      { value: 'portrait', ipp: 3 },
      { value: 'landscape', ipp: 4 }
    ];
    if (!list?.length) return all;
    const out = all.filter((o) => list.includes(o.ipp));
    return out.length ? out : all;
  }

  onMount(async () => {
    try {
      printers = await api.printers();
      cupsOk = true;
      if (printers.length) {
        printer = printers[0].name;
        await loadCaps();
      }
    } catch (e: any) {
      cupsOk = false;
      error = e.message;
    }
  });

  onDestroy(() => {
    if (url) revokeURL(url);
  });

  async function loadCaps() {
    if (!printer) return;
    try {
      const next = await api.caps(printer);
      caps = next;
      if (next.media_default) media = next.media_default;
      const so = supported(['none', 'fit', 'fill'], next.scaling_supported);
      if (!so.includes(scaling)) scaling = so.includes('fit') ? 'fit' : so[0];
      const co = supported(['color', 'monochrome'], next.color_mode_supported);
      if (!co.includes(color)) color = co[0];
      const oo = orientations(next.orientation_supported);
      if (!oo.some((o) => o.value === orientation)) orientation = oo[0].value;
    } catch (e: any) {
      error = e.message;
    }
  }

  function pick(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files?.length) setFile(input.files[0]);
  }

  function drop(e: DragEvent) {
    dragging = false;
    if (e.dataTransfer?.files.length) setFile(e.dataTransfer.files[0]);
  }

  function dragLeave(e: DragEvent) {
    const el = e.currentTarget as HTMLElement;
    if (!e.relatedTarget || !el.contains(e.relatedTarget as Node)) dragging = false;
  }

  function setFile(f: File) {
    error = '';
    if (!ALLOWED_MIME.has(f.type.toLowerCase()) && !NAME_OK.test(f.name)) {
      error = 'unsupported file type · use pdf, jpg, png, webp, or heic';
      return;
    }
    if (f.size > MAX_BYTES) {
      error = 'file too large · max 50 MB';
      return;
    }
    file = f;
    job = null;
    if (url) revokeURL(url);
    url = previewURL(f);
  }

  function clearFile() {
    file = null;
    job = null;
    if (url) revokeURL(url);
    url = null;
    if (inputEl) inputEl.value = '';
  }

  function clampCopies(value: unknown) {
    const n = Math.floor(Number(value));
    if (!Number.isFinite(n)) return 1;
    return Math.min(99, Math.max(1, n));
  }

  async function uploadAndPrint() {
    if (!file || !printer) return;
    busy = true;
    error = '';
    try {
      const created = await api.upload(file);
      job = await api.print(created.id, {
        printer,
        copies: clampCopies(copies),
        pageRanges,
        color,
        orientation,
        media,
        scaling
      });
      poll(job.id);
    } catch (e: any) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function poll(id: string) {
    for (let i = 0; i < 30; i++) {
      await new Promise((r) => setTimeout(r, 2000));
      try {
        job = await api.job(id);
        if (isTerminalStatus(job.status)) break;
      } catch {
        break;
      }
    }
  }

  async function cancel() {
    if (!job) return;
    try {
      job = await api.cancel(job.id);
    } catch (e: any) {
      error = e.message;
    }
  }
</script>

<Shell active="print" {chips}>
  {#if error}
    <div class="alert" role="alert">! {error}</div>
  {/if}

  <div class="grid">
    <section class="panel">
      <h2 class="panel-head">Source <span class="idx">01</span></h2>

      <button
        type="button"
        class="drop"
        class:drag={dragging}
        on:click={() => inputEl?.click()}
        on:dragover|preventDefault={() => (dragging = true)}
        on:dragleave={dragLeave}
        on:drop|preventDefault={drop}
      >
        <span class="plus">[ + ]</span>
        <strong>{file ? 'replace document' : 'drop file / click to browse'}</strong>
        <small>pdf · jpg · png · webp · heic / max 50 MB</small>
      </button>
      <input
        bind:this={inputEl}
        class="sr-only"
        type="file"
        accept={ACCEPT}
        tabindex="-1"
        on:change={pick}
      />

      {#if file}
        <div class="file-row">
          <span class="ext">{fileExt(file.name)}</span>
          <span class="name" title={file.name}>{file.name}</span>
          <span class="meta">{formatBytes(file.size)}</span>
          <button type="button" class="x" aria-label="Remove file" on:click={clearFile}>✕</button>
        </div>
      {/if}

      <div class="preview">
        {#if file && url}
          {#if isPDF(file)}
            <iframe src={url} title="Document preview"></iframe>
          {:else}
            <img src={url} alt="Document preview" />
          {/if}
        {:else}
          <span class="muted">[ no document loaded ]</span>
        {/if}
      </div>
    </section>

    <section class="panel">
      <h2 class="panel-head">Output <span class="idx">02</span></h2>

      {#if printers.length}
        <label class="field">
          <span class="label">printer</span>
          <select bind:value={printer} on:change={loadCaps}>
            {#each printers as p}
              <option value={p.name}>{p.name} · {p.state}</option>
            {/each}
          </select>
        </label>
      {:else}
        <div class="field">
          <span class="label">printer</span>
          <span class="muted">none configured · <a href="/admin/printers">add printer</a></span>
        </div>
      {/if}

      <label class="field">
        <span class="label">paper</span>
        <select bind:value={media}>
          {#each mediaOptions as m}
            <option value={m}>{mediaLabel(m)}</option>
          {/each}
        </select>
      </label>

      <div class="field">
        <span class="label" id="orientation-label">orientation</span>
        <div class="seg" role="group" aria-labelledby="orientation-label">
          {#each orientationOptions as o}
            <button
              type="button"
              class:on={orientation === o.value}
              aria-pressed={orientation === o.value}
              on:click={() => (orientation = o.value)}
            >{o.value}</button>
          {/each}
        </div>
      </div>

      <div class="field">
        <span class="label" id="ink-label">ink</span>
        <div class="seg" role="group" aria-labelledby="ink-label">
          {#each colorOptions as c}
            <button
              type="button"
              class:on={color === c}
              aria-pressed={color === c}
              on:click={() => (color = c)}
            >{c === 'monochrome' ? 'mono' : c}</button>
          {/each}
        </div>
      </div>

      <div class="field">
        <span class="label" id="scaling-label">scaling</span>
        <div class="seg" role="group" aria-labelledby="scaling-label">
          {#each scalingOptions as s}
            <button
              type="button"
              class:on={scaling === s}
              aria-pressed={scaling === s}
              on:click={() => (scaling = s)}
            >{s === 'none' ? 'actual' : s}</button>
          {/each}
        </div>
      </div>

      <label class="field">
        <span class="label">copies</span>
        <input type="number" min="1" max="99" bind:value={copies} />
      </label>

      <label class="field">
        <span class="label">pages</span>
        <input type="text" placeholder="all · e.g. 1-2,4" bind:value={pageRanges} />
      </label>

      <button type="button" class="btn btn-primary" disabled={!canPrint} on:click={uploadAndPrint}>
        {busy ? '> transmitting…' : '> PRINT_JOB --NOW'}
      </button>
      <p class="hint">uploaded files auto-delete after the retention window</p>
    </section>
  </div>

  {#if job}
    <JobSummary
      {job}
      onCancel={cancel}
      statusHref={`/job/${job.id}`}
    />
  {:else}
    <section class="panel">
      <h2 class="panel-head">Job queue <span class="idx">--</span></h2>
      <div class="empty">[ no active job ]</div>
    </section>
  {/if}
</Shell>
