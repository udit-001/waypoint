/**
 * Live-sync client (WP-144).
 *
 * One EventSource per app (topic is implicit — the DB change_events bus
 * is global; the API is kind-based). Subscribe with:
 *
 *   subscribeLive('matches', () => reloadQueue())
 *
 * Connection hygiene (ported from html-organizer fe822a1): the server is
 * plain HTTP 127.0.0.1 → HTTP/1.1, so the browser caps this origin at 6
 * parallel connections. Every open tab holding an EventSource pins one.
 * We hold it only while the tab is visible: hidden → close, resubscribe
 * on focus. The DB bus has no replay past the cursor, so a tab hidden
 * longer than RESYNC_MS resyncs once on return (re-running every
 * registered handler); quick switches skip the resync to avoid flicker.
 * Trade-off: events pushed while hidden are not seen live; the resync
 * covers data, not navigation.
 */

const RESYNC_MS = 5000;

const handlers = new Map(); // kind -> Set<fn>
let es = null;
let hiddenAt = 0;
let lastID = 0; // highest change-event ID seen; replay cursor on reconnect

function dispatch(ev) {
  const kind = ev && ev.kind;
  if (!kind) return;
  const set = handlers.get(kind);
  if (!set) return;
  for (const fn of set) {
    try {
      fn(ev);
    } catch (err) {
      console.error('[live] handler failed for', kind, err);
    }
  }
}

function subscribe() {
  if (es || document.hidden) return; // never hold a connection while hidden
  es = new EventSource(`/api/events?cursor=${lastID}`);
  es.addEventListener('message', (e) => {
    let ev;
    try {
      ev = JSON.parse(e.data);
    } catch {
      return;
    }
    if (ev.id && ev.id > lastID) lastID = ev.id;
    dispatch(ev);
  });
  es.onerror = () => {
    // Recreate with an updated cursor so a reconnect doesn't replay
    // every event since 0 (handler storm). EventSource reconnects
    // automatically, but only to the URL it was created with — so we
    // close and reopen with the advanced cursor.
    if (es) {
      es.close();
      es = null;
    }
    if (!document.hidden) {
      // Small backoff before re-subscribing; EventSource's own retry
      // would apply to the same URL, so we re-spawn manually.
      setTimeout(() => {
        if (!es && !document.hidden) subscribe();
      }, 1000);
    }
  };
}

function unsubscribe() {
  if (!es) return;
  es.close();
  es = null;
}

function resync() {
  for (const set of handlers.values()) {
    for (const fn of set) {
      try {
        fn({ kind: 'resync' });
      } catch (err) {
        console.error('[live] resync handler failed', err);
      }
    }
  }
}

document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    hiddenAt = Date.now();
    unsubscribe();
  } else {
    if (hiddenAt && Date.now() - hiddenAt > RESYNC_MS) resync();
    hiddenAt = 0;
    subscribe();
  }
});

/**
 * Register a handler for change events of `kind` ('matches', 'runs',
 * 'applications'). Also fires once for 'resync' when a long-hidden tab
 * returns (the DB bus has no replay). Returns an unsubscribe function.
 */
export function subscribeLive(kind, fn) {
  if (!handlers.has(kind)) handlers.set(kind, new Set());
  handlers.get(kind).add(fn);
  if (!es && !document.hidden) subscribe();
  return () => {
    const set = handlers.get(kind);
    if (set) set.delete(fn);
  };
}