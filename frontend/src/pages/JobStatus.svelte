<script lang="ts">
  import { onMount } from 'svelte';
  import Shell from '../lib/Shell.svelte';
  import JobSummary from '../lib/JobSummary.svelte';
  import { api, type Job } from '../lib/api';
  import { isTerminalStatus } from '../lib/format';

  export let id: string;

  let job: Job | null = null;
  let error = '';

  onMount(() => {
    const t = setInterval(async () => {
      try {
        job = await api.job(id);
        if (isTerminalStatus(job.status)) clearInterval(t);
      } catch (e: any) {
        error = e.message;
        clearInterval(t);
      }
    }, 2000);

    api.job(id).then((j) => (job = j)).catch((e: any) => (error = e.message));

    return () => clearInterval(t);
  });

  async function cancel() {
    try {
      job = await api.cancel(id);
    } catch (e: any) {
      error = e.message;
    }
  }
</script>

<Shell active="job">
  {#if error}
    <div class="alert" role="alert">! {error}</div>
  {/if}

  {#if job}
    <JobSummary title="Job status" {job} onCancel={cancel}>
      <a class="btn" href="/">← back to print</a>
    </JobSummary>
  {:else if !error}
    <section class="panel"><div class="empty">loading job…</div></section>
  {/if}
</Shell>
