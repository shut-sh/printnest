const CACHE = 'printnest-v3';
const SHARE_CACHE = 'printnest-share';
const SHARE_KEY = '/__printnest_share__';
const ASSETS = ['/', '/index.html', '/manifest.webmanifest', '/icon.svg', '/icon-192.png', '/icon-512.png', '/apple-touch-icon.png'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(ASSETS)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(
        keys.filter((k) => k !== CACHE && k !== SHARE_CACHE).map((k) => caches.delete(k))
      ))
      .then(() => self.clients.claim())
  );
});

async function handleShare(request) {
  try {
    const form = await request.formData();
    const files = form.getAll('file').filter((f) => typeof f === 'object' && f && 'name' in f && 'size' in f);
    const file = files[0];
    if (file) {
      const cache = await caches.open(SHARE_CACHE);
      await cache.put(
        SHARE_KEY,
        new Response(file, {
          headers: {
            'content-type': file.type || 'application/octet-stream',
            'x-printnest-filename': encodeURIComponent(file.name || 'shared')
          }
        })
      );
    }
  } catch {
    // A malformed share must still open the app.
  }
  return Response.redirect(new URL('/?share=1', self.location.origin).href, 303);
}

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);

  if (e.request.method === 'POST' && url.pathname === '/' && url.searchParams.has('share')) {
    e.respondWith(handleShare(e.request));
    return;
  }

  if (e.request.method !== 'GET' || url.pathname.startsWith('/api/')) return;

  e.respondWith(
    caches.match(e.request).then((hit) => hit || fetch(e.request).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(e.request, copy));
      return res;
    }).catch(() => caches.match('/index.html')))
  );
});
