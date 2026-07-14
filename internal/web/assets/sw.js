/* GoSplit service worker: caches the offline shell + static assets. Fingerprinted
 * /static/* assets (?v=<hash>) are cache-first — the URL changes when content
 * does, so a stale cache entry is never served for new content. Navigations and
 * other GETs stay network-first with cache fallback when offline. Bump CACHE on
 * any change to this file so old caches are purged on activate. */
const CACHE = 'gosplit-v3';
const SHELL = ['/offline', '/static/app.css', '/static/icon.svg', '/manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((keys) =>
    Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});

self.addEventListener('push', (e) => {
  let data = { title: 'GoSplit', body: 'You have an update.' };
  try { data = e.data.json(); } catch (_) {}
  e.waitUntil(self.registration.showNotification(data.title || 'GoSplit', {
    body: data.body || '',
    icon: '/static/icon.svg',
    badge: '/static/icon.svg',
    data: { url: data.url || '/' },
  }));
});

self.addEventListener('notificationclick', (e) => {
  e.notification.close();
  const url = (e.notification.data && e.notification.data.url) || '/';
  e.waitUntil(clients.matchAll({ type: 'window' }).then((wins) => {
    for (const w of wins) { if (w.url.includes(url) && 'focus' in w) return w.focus(); }
    if (clients.openWindow) return clients.openWindow(url);
  }));
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  if (req.mode === 'navigate') {
    e.respondWith(fetch(req).catch(() => caches.match('/offline')));
    return;
  }
  const url = new URL(req.url);
  if (url.origin === location.origin && url.pathname.startsWith('/static/')) {
    // Fingerprinted static assets: cache-first (URL changes when content does).
    e.respondWith(
      caches.match(req).then((hit) => hit || fetch(req).then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
        return res;
      }).catch(() => caches.match(req, { ignoreSearch: true }))),
    );
    return;
  }
  // Other GETs (e.g. /uploads): network-first so fresh content is picked up
  // right away; refresh the cache on success and fall back only when offline.
  e.respondWith(
    fetch(req).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
      return res;
    }).catch(() => caches.match(req)),
  );
});
