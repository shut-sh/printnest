const SHARE_CACHE = 'printnest-share';
const SHARE_KEY = '/__printnest_share__';

async function shareCache(): Promise<Cache | null> {
  if (!('caches' in window)) return null;
  return caches.open(SHARE_CACHE);
}

export async function hasPendingShare(): Promise<boolean> {
  const cache = await shareCache().catch(() => null);
  if (!cache) return false;
  return !!(await cache.match(SHARE_KEY));
}

export async function takeSharedFile(): Promise<File | null> {
  if (!new URLSearchParams(location.search).has('share')) return null;
  history.replaceState({}, '', location.pathname);

  const cache = await shareCache().catch(() => null);
  if (!cache) return null;
  const response = await cache.match(SHARE_KEY);
  if (!response) return null;
  await cache.delete(SHARE_KEY);

  const blob = await response.blob();
  const name = decodeURIComponent(response.headers.get('x-printnest-filename') || 'shared');
  const type = response.headers.get('content-type') || blob.type || 'application/octet-stream';
  return new File([blob], name, { type });
}
