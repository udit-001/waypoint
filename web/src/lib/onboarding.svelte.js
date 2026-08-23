// Setup-journey state shared by the Get Started surface, Applications'
// doorway, and the icon rail. Dismissal persists forever (seen-state);
// completion is derived by each surface from live API signals.

export const setup = $state({ dismissed: typeof localStorage !== 'undefined' && localStorage.getItem('wp_onboarding_dismissed') === '1' });

export function dismissSetup() {
  setup.dismissed = true;
  try { localStorage.setItem('wp_onboarding_dismissed', '1'); } catch { /* storage blocked */ }
}
