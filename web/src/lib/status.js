// Status display metadata — single source of truth for the frontend.
// STATUSES must match the DB seed (migrations/00001_init_schema.sql)
// and CLI stats order (internal/cli/stats.go).

export const STATUSES = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];

export const STATUS_STYLES = {
  'Not Applied': { bg: 'bg-slate-100 text-slate-600', border: 'border-slate-300' },
  'Applied':     { bg: 'bg-blue-100 text-blue-700',   border: 'border-blue-300' },
  'Offer':       { bg: 'bg-emerald-100 text-emerald-700', border: 'border-emerald-300' },
  'Rejected':    { bg: 'bg-red-100 text-red-700',     border: 'border-red-300' },
  'Withdrawn':   { bg: 'bg-slate-200 text-slate-500', border: 'border-slate-400' },
};
