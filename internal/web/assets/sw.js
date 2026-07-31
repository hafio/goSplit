/* GoSplit service worker: caches the offline shell + static assets. Fingerprinted
 * /static/* assets (?v=<hash>) are cache-first — the URL changes when content
 * does, so a stale cache entry is never served for new content. Navigations and
 * all other GETs are network-first with a NET_TIMEOUT cap: they always load live
 * from the server, cache a copy on success, and fall back to that cache only when
 * offline or when the network hangs past the timeout (navigations fall back to the
 * /offline shell if the page was never cached). Bump CACHE on any change to this
 * file so old caches are purged on activate. */
const CACHE = 'gosplit-v4';
const NET_TIMEOUT = 10000;
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

// fetch(req) but rejects if the network does not respond within `ms`, so a slow
// or hung connection falls back to cache instead of spinning indefinitely.
function networkWithTimeout(req, ms) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('network timeout')), ms);
    fetch(req).then(
      (res) => { clearTimeout(timer); resolve(res); },
      (err) => { clearTimeout(timer); reject(err); },
    );
  });
}

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
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
  // Navigations and all other GETs: always load live from the server (capped at
  // NET_TIMEOUT); refresh the cached copy on success and fall back to cache only
  // when offline or timed out — navigations use the /offline shell if uncached.
  e.respondWith(
    networkWithTimeout(req, NET_TIMEOUT).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
      return res;
    }).catch(() => caches.match(req).then((hit) =>
      hit || (req.mode === 'navigate' ? caches.match('/offline') : Response.error()),
    )),
  );
});
