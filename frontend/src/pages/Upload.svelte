<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Printer, type Caps, type Job } from '../lib/api';
  import { previewURL, revokeURL, isPDF } from '../lib/preview';

  let printers: Printer[] = [];
  let caps: Caps | null = null;
  let file: File | null = null;
  let url: string | null = null;
  let job: Job | null = null;
  let error = '';
  let busy = false;

  let printer = '';
  let copies = 1;
  let pageRanges = '';
  let color: string = 'color';
  let orientation: string = 'portrait';
  let media = 'iso_a4_210x297mm';
  let scaling: string = 'fit';

  onMount(async () => {
    try {
      printers = await api.printers();
      if (printers.length) {
        printer = printers[0].name;
        await loadCaps();
      }
    } catch (e: any) { error = e.message; }
  });

  async function loadCaps() {
    if (!printer) return;
    caps = await api.caps(printer);
    if (caps.media_default) media = caps.media_default;
  }

  function pick(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files?.length) setFile(input.files[0]);
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer?.files.length) setFile(e.dataTransfer.files[0]);
  }

  function setFile(f: File) {
    file = f; job = null; error = '';
    if (url) revokeURL(url);
    url = previewURL(f);
  }

  async function uploadAndPrint() {
    if (!file || !printer) return;
    busy = true; error = '';
    try {
      const j = await api.upload(file);
      job = await api.print(j.id, { printer, copies, pageRanges, color, orientation, media, scaling });
      poll(job.id);
    } catch (e: any) { error = e.message; }
    finally { busy = false; }
  }

  async function poll(id: string) {
    for (let i = 0; i < 30; i++) {
      await new Promise((r) => setTimeout(r, 2000));
      try {
        job = await api.job(id);
        if (job.status === 'done' || job.status === 'failed' || job.status === 'canceled') break;
      } catch { break; }
    }
  }

  async function cancel() {
    if (!job) return;
    try { job = await api.cancel(job.id); }
    catch (e: any) { error = e.message; }
  }
</script>

<main>
  <h1>PrintNest</h1>
  {#if error}<p class="err">{error}</p>{/if}

  <section class="drop" role="button" tabindex="0" on:dragover|preventDefault on:drop={drop}>
    <input type="file" accept="application/pdf,image/jpeg,image/png,image/webp,image/heic,image/heif,.pdf,.jpg,.jpeg,.png,.webp,.heic,.heif" on:change={pick} />
    <p>{file ? file.name : 'Choose / Share / drop a file (PDF, JPG, PNG, WEBP, HEIC, max 50 MB)'}</p>
  </section>

  {#if file && url}
    <section>
      <h2>Preview</h2>
      {#if isPDF(file)}
        <iframe src={url} title="preview" width="100%" height="480"></iframe>
      {:else}
        <img src={url} alt="preview" />
      {/if}
    </section>
  {/if}

  <section>
    <h2>Settings</h2>
    <label>Printer
      <select bind:value={printer} on:change={loadCaps}>
        {#each printers as p}<option value={p.name}>{p.name} ({p.state})</option>{/each}
      </select>
    </label>
    <label>Paper
      <select bind:value={media}>
        {#if caps?.media_supported?.length}
          {#each caps.media_supported as m}<option value={m}>{m}</option>{/each}
        {:else}
          <option value="iso_a4_210x297mm">A4</option>
        {/if}
      </select>
    </label>
    <div class="row">
      <label><input type="radio" bind:group={orientation} value="portrait" /> Portrait</label>
      <label><input type="radio" bind:group={orientation} value="landscape" /> Landscape</label>
    </div>
    <div class="row">
      <label><input type="radio" bind:group={color} value="color" /> Color</label>
      <label><input type="radio" bind:group={color} value="monochrome" /> Black &amp; White</label>
    </div>
    <label>Scaling
      <select bind:value={scaling}>
        <option value="none">Actual size</option>
        <option value="fit">Fit to page</option>
        <option value="fill">Fill page</option>
      </select>
    </label>
    <label>Copies <input type="number" min="1" bind:value={copies} /></label>
    <label>Page range <input placeholder="1-2" bind:value={pageRanges} /></label>
    <button disabled={!file || !printer || busy} on:click={uploadAndPrint}>{busy ? 'Printing...' : 'Print'}</button>
  </section>

  {#if job}
    <section>
      <h2>Job {job.status}</h2>
      <p>{job.orig_name} - CUPS #{job.cups_job_id}</p>
      {#if job.error}<p class="err">{job.error}</p>{/if}
      {#if job.status === 'pending' || job.status === 'processing'}
        <button on:click={cancel}>Cancel</button>
      {/if}
      <p><a href="/job/{job.id}">Open status page</a></p>
    </section>
  {/if}

  <nav><a href="/admin/printers">Printers</a> | <a href="/admin/settings">Settings</a></nav>
</main>

<style>
  main { max-width: 40rem; margin: 0 auto; padding: 1rem; font-family: system-ui, sans-serif; }
  .drop { border: 2px dashed #888; padding: 1.5rem; text-align: center; }
  img { max-width: 100%; }
  label { display: block; margin: .6rem 0; }
  input, select, button { font-size: 1rem; padding: .5rem; max-width: 100%; }
  .row { display: flex; gap: 1rem; }
  .err { color: #b00020; }
</style>
