// Canonical skillId → {label, icon, tags} mapping.
// Single source of truth — every skill consumer imports from here.
// icon names reference keys in lib/icons.js

export const skillMeta = {
  'email-generator': { label: 'Email', icon: 'mail', tags: ['7 email types', '4 tones', 'Auto-signature'] },
  'cover-letter': { label: 'Cover Letter', icon: 'file-text', tags: ['4 tones', '3 lengths', 'Skill emphasis'] },
  'resume-optimizer': { label: 'Resume Optimizer', icon: 'search', tags: ['Match score', 'Gap analysis', 'Action verbs'] },
  'interview-prep': { label: 'Interview Prep', icon: 'target', tags: ['6 interview types', '3 difficulty levels', 'Research checklist'] },
  'career-summary': { label: 'Career Summary', icon: 'star', tags: ['5 styles', '3 lengths', 'Target role'] },
  'statement-of-purpose': { label: 'SOP', icon: 'grad', tags: ['4 tones', '3 lengths', 'SOP checklist'] },
};

/** Get a human-readable label for a skillId. Falls back to the raw id. */
export function skillLabel(id) {
  return skillMeta[id]?.label ?? id;
}

/** Get the Lucide icon name for a skillId. Falls back to 'file'. */
export function skillIcon(id) {
  return skillMeta[id]?.icon ?? 'file';
}

/** Get tags for a skillId. Falls back to empty array. */
export function skillTags(id) {
  return skillMeta[id]?.tags ?? [];
}
