<script>
  // Get Started — the setup journey on its own surface (no longer
  // borrowed from Applications). Same live-signal checklist, same
  // seen-state rules; the rail entry disappears when it stops applying.
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import * as api from '../stores/api.svelte.js';
  import { getRouter } from '../stores/router.svelte.js';
  import { setup, dismissSetup } from '../lib/onboarding.svelte.js';

  const router = getRouter();

  let profileName = $state('');
  let autopilotRan = $state(false);
  let zenKeySet = $state(false);

  let allJobs = $state([]);
  let copiedPrompt = $state(false);

  const WELCOME_PROMPT =
    "Set up my job search on Waypoint.\n" +
    "If you don't have the waypoint skill installed yet, install it first with `waypoint skills install`.\n" +
    "Then interview me about what I'm looking for, save my profile, and start finding jobs for me to review.";

  function copyPrompt() {
    try {
      navigator.clipboard.writeText(WELCOME_PROMPT);
      copiedPrompt = true;
      setTimeout(() => { copiedPrompt = false; }, 1500);
    } catch { /* clipboard blocked */ }
  }

  const setupSteps = $derived([
    {
      id: 'profile',
      title: 'Save your profile',
      why: 'The first thing your assistant reads — it drives matching.',
      cta: 'Open Profile',
      href: '/profile',
      done: profileName !== '',
    },
    {
      id: 'assistant',
      title: 'Hand the setup prompt to your assistant',
      why: 'Your assistant installs the skill and starts the search.',
      cta: 'Copy prompt',
      copy: true,
      done: autopilotRan,
    },
    {
      id: 'keys',
      title: 'Connect scoring — paste your Zen key',
      why: 'One free key makes autopilot judge every posting before it reaches Matches.',
      cta: 'Open Settings',
      href: '/settings',
      done: zenKeySet,
    },
    {
      id: 'review',
      title: 'Review your first matches',
      why: 'Autopilot files the best ones in Matches — you Add or Dismiss.',
      cta: 'Open Matches',
      href: '/found',
      done: allJobs.length > 0,
    },
  ]);
  const setupDone = $derived(setupSteps.filter(s => s.done).length);
  const setupComplete = $derived(setupDone === setupSteps.length);

  $effect(() => {
    setPage({ title: 'Get started', byline: `${setupDone} of ${setupSteps.length} done` });
  });

  // Retire the surface the moment it stops applying — skip or full
  // completion both hand back to Applications.
  $effect(() => {
    if (setup.dismissed) router.navigate('/applications');
    if (setupComplete) dismissSetup();
  });

  api.jobs.ensure().then(() => { allJobs = api.jobs.value || []; }).catch(() => {});
  api.profile.ensure().then(() => { profileName = api.profile.value?.name || ''; }).catch(() => {});
  api.autopilot.ensure().then(() => {
    autopilotRan = !!api.autopilot.value?.lastRun;
    zenKeySet = !!api.autopilot.value?.zenKeySet;
  }).catch(() => {});
</script>

<div class="max-w-md mx-auto py-12 px-4">
  <h3 class="text-xl font-semibold text-slate-800 dark:text-slate-200 mb-1">Set up your job search</h3>
  <p class="text-sm text-slate-500 dark:text-slate-400 mb-6">Then Waypoint runs itself. Your assistant does the legwork.</p>

  <div class="bg-white dark:bg-slate-700/60 border border-slate-200 dark:border-slate-600 rounded-xl divide-y divide-slate-100 dark:divide-slate-600">
    {#each setupSteps as step, i (step.id)}
      {@const isNext = !step.done && setupSteps.slice(0, i).every(s => s.done)}
      <div class="flex items-start gap-3 px-4 py-3.5 {step.done || isNext ? '' : 'opacity-50'}">
        {#if step.done}
          <span class="shrink-0 mt-0.5 text-emerald-600 dark:text-emerald-400">{@html iconSvg('check-circle', 18)}</span>
        {:else}
          <span class="shrink-0 mt-0.5 size-[18px] rounded-full border-2 {isNext ? 'border-slate-400 dark:border-slate-400' : 'border-slate-200 dark:border-slate-600'} flex items-center justify-center text-[10px] font-semibold text-slate-400">{i + 1}</span>
        {/if}
        <div class="flex-1 min-w-0">
          <p class="text-sm font-medium {step.done ? 'text-slate-400 dark:text-slate-500 line-through' : 'text-slate-800 dark:text-slate-100'}">{step.title}</p>
          <p class="text-xs text-slate-400 dark:text-slate-500 mt-0.5">{step.why}</p>
        </div>
        {#if !step.done && isNext}
          {#if step.copy}
            <button
              class="shrink-0 mt-0.5 px-3 py-1.5 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors cursor-pointer {copiedPrompt ? 'emerald-check' : ''}"
              onclick={copyPrompt}
            >{#if copiedPrompt}<span class="inline-flex items-center gap-1">{@html iconSvg('check', 12)}Copied</span>{:else}{step.cta}{/if}</button>
          {:else}
            <a
              href={step.href}
              class="shrink-0 mt-0.5 px-3 py-1.5 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors"
            >{step.cta}</a>
          {/if}
        {/if}
      </div>
    {/each}
  </div>

  <div class="flex items-center justify-between mt-4">
    <span class="text-xs text-slate-400 dark:text-slate-500 tabular-nums">{setupDone} of {setupSteps.length} done</span>
    <button
      class="text-xs text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer bg-transparent border-none p-0"
      onclick={dismissSetup}
    >Skip — I'll explore on my own.</button>
  </div>
</div>
