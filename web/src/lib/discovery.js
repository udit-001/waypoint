// Pure helpers for the Matches discovery band (WP-154).

// candidateBoardLabel renders one candidate board as "provider/token" —
// the same token that matters per provider in the CLI table: tenant for
// Eightfold, site slug for Workday, board token/org for the rest.
export function candidateBoardLabel(board) {
  let part = '';
  try {
    const u = new URL(board.url);
    if (board.provider === 'eightfold') {
      part = u.hostname.replace(/\.eightfold\.ai$/, '');
    } else {
      part = u.pathname.replace(/^\/+|\/+$/g, '').split('/')[0] || '';
    }
  } catch {
    part = '';
  }
  return `${board.provider}/${part}`;
}

// pendingCandidates narrows a candidates list to the review queue.
export function pendingCandidates(cands) {
  return (cands || []).filter(c => c.status === 'suggested');
}
