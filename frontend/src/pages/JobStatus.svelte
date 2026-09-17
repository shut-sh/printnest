<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Job } from '../lib/api';
  export let id: string;
  let job: Job | null = null;
  let error = '';
  onMount(async () => {
    const t = setInterval(async () => {
      try {
        job = await api.job(id);
        if (job.status === 'done' || job.status === 'failed' || job.status === 'canceled') clearInterval(t);
      } catch (e: any) { error = e.message; clearInterval(t); }
    }, 2000);
    try { job = await api.job(id); } catch (e: any) { error = e.message; }
    return () => clearInterval(t);
  });
  async function cancel() {
    try { job = await api.cancel(id); }
    catch (e: any) { error = e.message; }
  }
</script>

<main>
  <h1>Job status</h1>
  {#if error}<p class="err">{error}</p>{/if}
  {#if job}
    <p>{job.orig_name} - <strong>{job.status}</strong></p>
    <p>Printer: {job.printer} - CUPS #{job.cups_job_id}</p>
    {#if job.error}<p class="err">{job.error}</p>{/if}
    {#if job.status === 'pending' || job.status === 'processing'}
      <button on:click={cancel}>Cancel job</button>
    {/if}
  {/if}
  <p><a href="/">Back</a></p>
</main>

<style>
  main { max-width: 40rem; margin: 0 auto; padding: 1rem; font-family: system-ui, sans-serif; }
  .err { color: #b00020; }
</style>
