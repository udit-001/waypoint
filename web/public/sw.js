// Waypoint service worker — caches the app shell + offline page.
// The app is server-backed (SQLite + Go), so "offline" means "server not
// running," not "no network." The SW guards origin identity: if a foreign
// app occupies the port, the sentinel check falls back to the cached
// offline page instead of rendering the foreign response.
//
// Vite produces hashed asset filenames (e.g. /assets/index-AbC123.js),
// so hashed JS/CSS cannot be precached — they are runtime-cached on
// first fetch via the cache-first strategy below. Only stable files
// (offline page, manifest, icons) are precached on install.

var CACHE = 'waypoint-v1';

var PRECACHE = [
  '/offline.html',
  '/manifest.json',
  '/icons/icon-192.svg',
  '/icons/icon-512.svg',
  '/icons/icon-192.png',
  '/icons/icon-512.png'
];

self.addEventListener('install', function(e) {
  e.waitUntil(
    caches.open(CACHE).then(function(cache) {
      return cache.addAll(PRECACHE);
    }).then(function() {
      return self.skipWaiting();
    })
  );
});

self.addEventListener('activate', function(e) {
  e.waitUntil(
    caches.keys().then(function(keys) {
      return Promise.all(
        keys.filter(function(k) { return k !== CACHE; })
            .map(function(k) { return caches.delete(k); })
      );
    }).then(function() {
      return self.clients.claim();
    })
  );
});

var SENTINEL = 'name="waypoint-app"';

self.addEventListener('fetch', function(e) {
  var req = e.request;

  // Navigations: network-first with identity guard.
  // Intercepts top-level document navigations. If the response doesn't
  // contain the sentinel meta tag (e.g. a foreign app hijacked the port),
  // serves the cached offline page instead. On network failure (server
  // down), falls back to cached app shell, then offline page.
  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req).then(function(resp) {
        var clone = resp.clone();
        return clone.text().then(function(body) {
          if (body.indexOf(SENTINEL) !== -1) {
            // Cache the valid app shell for potential reuse.
            var cacheResp = resp.clone();
            caches.open(CACHE).then(function(cache) { cache.put('/', cacheResp); });
            return resp;
          }
          // Foreign response on our port — serve offline page instead.
          return caches.match('/offline.html').then(function(offline) {
            return offline || resp;
          });
        });
      }).catch(function() {
        // Server down — try cached app shell, then offline page.
        return caches.match('/').then(function(cached) {
          return cached || caches.match('/offline.html');
        }).then(function(fallback) {
          return fallback || Response.error();
        });
      })
    );
    return;
  }

  var url = new URL(req.url);

  // Same-origin static assets (Vite JS/CSS, fonts, images): cache-first.
  // This captures hashed bundles at runtime — first fetch populates the
  // cache, subsequent offline loads serve from cache.
  if (url.origin === self.location.origin) {
    if (req.destination === 'style' || req.destination === 'script' ||
        req.destination === 'image' || req.destination === 'font' ||
        url.pathname.startsWith('/assets/') || url.pathname.startsWith('/icons/')) {
      e.respondWith(
        caches.match(req).then(function(cached) {
          if (cached) return cached;
          return fetch(req).then(function(resp) {
            if (resp.ok) {
              var clone = resp.clone();
              caches.open(CACHE).then(function(cache) { cache.put(req, clone); });
            }
            return resp;
          });
        })
      );
      return;
    }
  }

  // Everything else (API, POST, etc.): pass through.
});
