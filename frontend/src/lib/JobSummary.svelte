<script lang="ts">
  import JobTimeline from './JobTimeline.svelte';
  import type { Job } from './api';
  import {
    formatTimestamp,
    isCancelableStatus,
    statusLabel,
    statusTone
  } from './format';

  export let job: Job;
  export let title = 'Job queue';
  export let statusHref = '';
  export let onCancel: (() => void) | null = null;
</script>

<section class="panel" aria-live="polite">
  <h2 class="panel-head">
    {title}
    {#if job.cups_job_id}<span class="muted">#{job.cups_job_id}</span>{/if}
    <span class="chip {statusTone(job.status)}">{statusLabel(job.status)}</span>
  </h2>

  <JobTimeline status={job.status} />

  <dl class="meta-grid">
    <div><dt>document</dt><dd>{job.orig_name}</dd></div>
    <div><dt>printer</dt><dd>{job.printer || '—'}</dd></div>
    <div><dt>cups job</dt><dd>{job.cups_job_id ? `#${job.cups_job_id}` : '—'}</dd></div>
    <div><dt>copies</dt><dd>{job.copies}</dd></div>
    <div><dt>mode</dt><dd>{job.color === 'monochrome' ? 'mono' : 'color'} · {job.scaling}</dd></div>
    <div><dt>pages</dt><dd>{job.page_ranges || 'all'}</dd></div>
    <div><dt>submitted</dt><dd>{formatTimestamp(job.created_at)}</dd></div>
    <div><dt>job id</dt><dd>{job.id}</dd></div>
  </dl>

  {#if job.error}
    <div class="alert" style="margin-top:14px">! {job.error}</div>
  {/if}

  <div class="row-actions">
    {#if onCancel && isCancelableStatus(job.status)}
      <button type="button" class="btn" on:click={onCancel}>cancel job</button>
    {/if}
    {#if statusHref}
      <a class="btn" href={statusHref}>open status page →</a>
    {/if}
    <slot />
  </div>
</section>
