<script lang="ts">
  import Login from './pages/Login.svelte';
  import Upload from './pages/Upload.svelte';
  import JobStatus from './pages/JobStatus.svelte';
  import AdminPrinters from './pages/AdminPrinters.svelte';
  import AdminSettings from './pages/AdminSettings.svelte';

  let path = location.pathname;
  let jobId = '';
  const m = path.match(/^\/job\/(.+)$/);
  if (m) jobId = decodeURIComponent(m[1]);
</script>

{#if path === '/login'}
  <Login />
{:else if path.startsWith('/job/')}
  <JobStatus id={jobId} />
{:else if path === '/admin/printers'}
  <AdminPrinters />
{:else if path === '/admin/settings'}
  <AdminSettings />
{:else}
  <Upload />
{/if}
