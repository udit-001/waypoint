<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import * as api from '../stores/api.svelte.js';

  let profileData = $state(null);

  onMount(async () => {
    setPage({ title: 'Profile' });

    await api.profile.ensure();
    profileData = api.profile.value;
  });

  function esc(str) {
    if (!str) return '-';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }
</script>

<div class="space-y-4">
  <p class="text-sm text-slate-400 mb-4">
    Your profile personalizes AI-generated content. Manage it via the CLI.
  </p>

  {#if profileData}
    <!-- Personal Info -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        {@html iconSvg("user", 18)} Personal Info
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Full Name</label>
          <div class="text-sm text-slate-700">{profileData.name || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Professional Title</label>
          <div class="text-sm text-slate-700">{profileData.title || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Email</label>
          <div class="text-sm text-slate-700">{profileData.email || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Phone</label>
          <div class="text-sm text-slate-700">{profileData.phone || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Industry</label>
          <div class="text-sm text-slate-700">{profileData.industry || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Greeting Style</label>
          <div class="text-sm text-slate-700 capitalize">{profileData.greetingStyle || 'formal'}</div>
        </div>
      </div>
    </Card>

    <!-- Skills -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        <span>{@html iconSvg('zap', 18)}</span> Skills
      </h3>
      {#if profileData.skills?.length}
        <div class="flex flex-wrap gap-1.5">
          {#each profileData.skills as skill}
            <span class="bg-slate-100 border border-slate-200 text-slate-500 rounded-full px-2.5 py-0.5 text-xs">{skill}</span>
          {/each}
        </div>
      {:else}
        <p class="text-sm text-slate-400">No skills set yet.</p>
      {/if}
    </Card>

    <!-- Education -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        {@html iconSvg('grad', 18)} Education
      </h3>
      {#if profileData.education?.length}
        <ul class="list-disc pl-5 text-sm text-slate-700 space-y-1">
          {#each profileData.education as item}
            <li>{item}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-slate-400">No education set yet.</p>
      {/if}
    </Card>

    <!-- Experience -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        {@html iconSvg('briefcase', 18)} Experience
      </h3>
      {#if profileData.experience?.length}
        <ul class="list-disc pl-5 text-sm text-slate-700 space-y-1">
          {#each profileData.experience as item}
            <li>{item}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-slate-400">No experience set yet.</p>
      {/if}
    </Card>

    <!-- Email Preferences -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        {@html iconSvg('mail', 18)} Email Preferences
      </h3>
      <div>
        <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Sign-Off</label>
        <div class="text-sm text-slate-700">{profileData.signOff || 'Best regards'}</div>
      </div>
    </Card>
  {:else if api.profile.loading}
    <Spinner text="Loading profile..." />
  {:else}
    <p class="text-sm text-slate-400">Profile not loaded (server may be down).</p>
  {/if}
</div>
