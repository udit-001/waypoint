// deadlineUrgency returns a relative label for a deadline date.
// Shared across Dashboard, TableView, and Kanban for consistent urgency signaling.

export function deadlineDaysLeft(dateStr) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  if (isNaN(d)) return null;
  return Math.ceil((d - new Date()) / 86400000);
}

// deadlineLevel is the single source of truth for urgency thresholds.
// All presentation functions consume this — change the boundary here, not in callers.
// Returns: 'overdue' | 'imminent' | 'normal' | null
function deadlineLevel(days) {
  if (days === null) return null;
  if (days < 0) return 'overdue';
  if (days <= 7) return 'imminent';
  return 'normal';
}

export function deadlineLabel(days) {
  const level = deadlineLevel(days);
  if (level === null) return '';
  if (level === 'overdue') return `${Math.abs(days)}d overdue`;
  if (days === 0) return 'Today';
  return `${days}d`;
}

export function deadlineClass(days) {
  const level = deadlineLevel(days);
  if (level === 'overdue') return 'text-red-600';
  if (level === 'imminent') return 'text-amber-600';
  return 'text-emerald-600';
}

// deadlineClassMuted is for compact surfaces (Kanban cards) where only
// overdue and imminent deadlines should draw attention. Normal deadlines
// stay in the same muted tone as surrounding metadata — no green.
export function deadlineClassMuted(days) {
  const level = deadlineLevel(days);
  if (level === 'overdue') return 'text-red-600';
  if (level === 'imminent') return 'text-amber-600';
  return '';
}

// Row-level tint for table rows where action is needed.
// Only applies to overdue or imminent deadlines on active jobs.
export function deadlineRowTint(days, status) {
  const level = deadlineLevel(days);
  if (level === null) return '';
  if (status === 'Offer' || status === 'Rejected' || status === 'Withdrawn') return '';
  if (level === 'overdue') return 'bg-tint-red';
  if (level === 'imminent') return 'bg-tint-amber';
  return '';
}
