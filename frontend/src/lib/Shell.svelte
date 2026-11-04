<script lang="ts">
  import type { Tone } from './format';

  export let active: 'print' | 'printers' | 'settings' | 'login' | 'job' = 'print';
  export let chips: { label: string; tone?: Tone }[] = [];

  const label: Record<string, string> = {
    print: 'print --interactive',
    printers: 'printers --manage',
    settings: 'settings',
    login: 'login',
    job: 'job --status'
  };

  const title: Record<string, string> = {
    print: 'Print',
    printers: 'Printers',
    settings: 'Settings',
    login: 'Sign in',
    job: 'Job status'
  };
</script>

<header class="topbar">
  <div class="topbar-inner">
    <a class="brand" href="/">PRINT<span>NEST</span></a>
    <span class="prompt">printnest@local:~$ <b>{label[active]}</b></span>
    <span class="spacer"></span>

    {#if chips.length}
      <div class="chips">
        {#each chips as chip}
          <span class="chip {chip.tone ?? ''}">
            {#if chip.tone}<span class="dot"></span>{/if}{chip.label}
          </span>
        {/each}
      </div>
    {/if}

    {#if active !== 'login'}
      <nav class="nav" aria-label="Main">
        <a href="/" class:active={active === 'print'} aria-current={active === 'print' ? 'page' : undefined}>print</a>
        <a href="/admin/printers" class:active={active === 'printers'} aria-current={active === 'printers' ? 'page' : undefined}>printers</a>
        <a href="/admin/settings" class:active={active === 'settings'} aria-current={active === 'settings' ? 'page' : undefined}>settings</a>
      </nav>
    {/if}
  </div>
</header>

<h1 class="sr-only">{title[active]}</h1>

<main class="wrap">
  <slot />
</main>

<footer class="foot">
  <span>cups · ipp · pdf</span>
  <span>self-hosted</span>
  <span>tls via reverse proxy</span>
  <a href="https://github.com/shut-sh/printnest">source</a>
</footer>
