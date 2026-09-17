CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  printer TEXT NOT NULL DEFAULT '',
  orig_name TEXT NOT NULL,
  mime TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  copies INTEGER NOT NULL DEFAULT 1,
  page_ranges TEXT NOT NULL DEFAULT '',
  color_mode TEXT NOT NULL DEFAULT 'color',
  orientation TEXT NOT NULL DEFAULT 'portrait',
  media TEXT NOT NULL DEFAULT 'iso_a4_210x297mm',
  scaling TEXT NOT NULL DEFAULT 'fit',
  status TEXT NOT NULL DEFAULT 'uploaded',
  cups_job_id INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  printed_at INTEGER NOT NULL DEFAULT 0,
  file_deleted_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS printers_cache (
  name TEXT PRIMARY KEY,
  attrs TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
