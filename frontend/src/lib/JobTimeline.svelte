<script lang="ts">
  import { timelineIndex } from './format';

  export let status: string;

  const steps = ['upload', 'convert', 'transmit', 'spool', 'done'];

  $: failed = status === 'failed';
  $: current = timelineIndex(status);
</script>

<div class="steps" aria-hidden="true">
  {#each steps as step, i}
    <div
      class="step"
      class:done={!failed && i < current}
      class:cur={!failed && i === current}
      class:bad={failed && i === 2}
    >{step}</div>
  {/each}
</div>
