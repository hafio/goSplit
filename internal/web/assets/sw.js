/* GoSplit service worker: caches the offline shell + static assets. Network-
 * first for both navigations and static assets (so a redeploy's fresh CSS/JS is
 * picked up immediately), with cache fallback when offline. Bump CACHE on any
 * change to this file so old caches are purged on activate. */
const CACHE = 'gosplit-v2';
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
  // Static assets: network-first so redeploys are picked up right away; refresh
  // the cache on success and fall back to it only when the network fails.
  e.respondWith(
    fetch(req).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
      return res;
    }).catch(() => caches.match(req)),
  );
});
