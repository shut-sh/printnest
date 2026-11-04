export type Tone = 'ok' | 'warn' | 'bad' | '';

export function formatBytes(n: number): string {
  if (n === 0) return '0 B';
  if (!n) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = n;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value >= 10 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

export function fileExt(name: string): string {
  const m = name.match(/\.([a-z0-9]+)$/i);
  return m ? m[1].toUpperCase() : 'FILE';
}

export function mediaLabel(media: string): string {
  switch (media) {
    case 'iso_a4_210x297mm': return 'A4 · 210×297 mm';
    case 'na_letter_8.5x11in': return 'Letter · 8.5×11 in';
    case 'na_legal_8.5x14in': return 'Legal · 8.5×14 in';
    default: return media.replace(/_/g, ' ');
  }
}

export function formatTimestamp(unix: number): string {
  return unix ? new Date(unix * 1000).toLocaleString() : '—';
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'uploaded': return 'uploaded';
    case 'pending': return 'queued';
    case 'processing': return 'processing';
    case 'done': return 'done';
    case 'failed': return 'failed';
    case 'canceled': return 'canceled';
    default: return status;
  }
}

export function statusTone(status: string): Tone {
  if (status === 'done') return 'ok';
  if (status === 'failed') return 'bad';
  if (status === 'canceled') return '';
  return 'warn';
}

export function isTerminalStatus(status: string): boolean {
  return status === 'done' || status === 'failed' || status === 'canceled';
}

export function isCancelableStatus(status: string): boolean {
  return status === 'pending' || status === 'processing';
}

export function timelineIndex(status: string): number {
  switch (status) {
    case 'uploaded': return 1;
    case 'pending': return 2;
    case 'processing': return 3;
    case 'done': return 5;
    default: return -1;
  }
}

export function printerStateTone(state: string): Tone {
  if (state === 'idle') return 'ok';
  if (state === 'stopped') return 'bad';
  return 'warn';
}
