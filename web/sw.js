const CACHE_NAME = 'job-tracker-v2';
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/css/style.css',
  '/css/fonts.css',
  '/fonts/pt-serif-regular-400.ttf',
  '/fonts/pt-serif-bold-700.ttf',
  '/fonts/pt-serif-italic-400.ttf',
  '/fonts/pt-serif-italic-700.ttf',
  '/vendor/chart.umd.min.js',
  '/vendor/marked.min.js',
  '/js/icons.js',
  '/js/data.js',
  '/js/ui.js',
  '/js/app.js',
  '/js/dashboard.js',
  '/js/kanban.js',
  '/js/table.js',
  '/js/timeline.js',
  '/js/search.js',
  '/js/notes.js',
  '/js/generated.js',
  '/js/export.js',
  '/js/settings.js',
  '/js/notifications.js',
  '/js/skills.js',
  '/manifest.json',
  '/icons/icon-192.svg',
  '/icons/icon-512.svg',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  event.respondWith(networkFirst(request));
});

async function networkFirst(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    if (cached) return cached;
    if (request.url.includes('/api/')) {
      const isHistory = request.url.includes('/history');
      const isStats = request.url.includes('/stats');
      return new Response(
        JSON.stringify(
          isStats
            ? { total: 0, byStatus: {}, byCategory: {} }
            : isHistory ? [] : []
        ),
        { headers: { 'Content-Type': 'application/json' } }
      );
    }
    return new Response('Offline', { status: 503 });
  }
}
