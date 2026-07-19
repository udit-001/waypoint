// Status display metadata — single source of truth for the frontend.
// STATUSES must match the DB seed (migrations/00001_init_schema.sql)
// and CLI stats order (internal/cli/stats.go).

export const STATUSES = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];

// STATUS_META is the canonical source for status visual identity.
// - color: hex used for inline-style icon strokes + dot backgrounds
// - icon:  lucide icon name (resolves via iconSvg in lib/icons.js)
// - bg:    Tailwind classes for pill badges (JobDetail)
// - border: Tailwind border class for pill badges
//
// The icon is the canonical visual marker. The color is the canonical
// hue. Together they encode status identity across every surface.
export const STATUS_META = {
  'Not Applied': { color: '#94a3b8', icon: 'circle-dashed',  bg: 'bg-slate-100 text-slate-600',           border: 'border-slate-300' },
  'Applied':     { color: '#5e81ac', icon: 'send',           bg: 'bg-blue-100 text-blue-700',            border: 'border-blue-300' },
  'Offer':       { color: '#a3be8c', icon: 'award',          bg: 'bg-emerald-100 text-emerald-700',       border: 'border-emerald-300' },
  'Rejected':    { color: '#bf616a', icon: 'circle-x',      bg: 'bg-red-100 text-red-700',              border: 'border-red-300' },
  'Withdrawn':   { color: '#7b8794', icon: 'circle-arrow-left', bg: 'bg-slate-200 text-slate-500',       border: 'border-slate-400' },
};
