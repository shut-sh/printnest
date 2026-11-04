export interface Printer {
  name: string;
  make_model: string;
  state: string;
  state_reasons: string[];
  device_uri: string;
  uri: string;
  accepting?: boolean;
}

export interface Device {
  uri: string;
  make_model: string;
  info: string;
  class: string;
}

export interface Caps {
  media_supported: string[];
  media_default: string;
  color_mode_supported: string[];
  scaling_supported: string[];
  orientation_supported: number[];
  copies_supported: boolean;
  page_ranges_supported: boolean;
  document_format_supported: string[];
  printer_state: string;
  state_reasons: string[];
}

export interface Job {
  id: string;
  printer: string;
  orig_name: string;
  mime: string;
  copies: number;
  page_ranges: string;
  color: string;
  orientation: string;
  media: string;
  scaling: string;
  status: string;
  cups_job_id: number;
  error: string;
  created_at: number;
}

async function req(path: string, opts: RequestInit = {}): Promise<any> {
  const res = await fetch(path, { ...opts, credentials: 'same-origin' });
  if (res.status === 401) {
    if (location.pathname !== '/login') location.href = '/login';
    throw new Error('unauthorized');
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data as any).error || `request failed ${res.status}`);
  return data;
}

export const api = {
  login: (password: string) =>
    req('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ password }) }),
  logout: () => req('/api/logout', { method: 'POST' }),
  printers: async (): Promise<Printer[]> => (await req('/api/printers')).printers ?? [],
  discover: async (): Promise<Device[]> => (await req('/api/printers/discover', { method: 'POST' })).devices ?? [],
  addPrinter: (name: string, uri: string, model: string) =>
    req('/api/printers/add', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, uri, model }) }),
  caps: (name: string): Promise<Caps> => req(`/api/printers/${encodeURIComponent(name)}/caps`),
  upload: async (file: File): Promise<Job> => {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch('/api/jobs', { method: 'POST', body: fd, credentials: 'same-origin' });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error((data as any).error || `upload failed ${res.status}`);
    return data as Job;
  },
  print: (id: string, body: object): Promise<Job> =>
    req(`/api/jobs/${id}/print`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  job: (id: string): Promise<Job> => req(`/api/jobs/${id}`),
  jobs: async (limit = 50): Promise<Job[]> => (await req(`/api/jobs?limit=${limit}`)).jobs ?? [],
  cancel: (id: string): Promise<Job> => req(`/api/jobs/${id}/cancel`, { method: 'POST' }),
  settings: (): Promise<{ file_retention_minutes: number }> => req('/api/settings'),
  saveSettings: (file_retention_minutes: number) =>
    req('/api/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ file_retention_minutes }) }),
};
