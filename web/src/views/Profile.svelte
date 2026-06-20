<script>
  import { onMount } from 'svelte';
  import * as api from '../stores/api.js';

  let profileData = $state(null);

  onMount(async () => {
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

<div class="max-w-3xl mx-auto space-y-4">
  <p class="text-sm text-slate-400 mb-4">
    Your profile personalizes AI-generated content. Manage it via the CLI.
  </p>

  <!-- CLI Commands -->
  <div class="bg-white rounded-xl border border-slate-200 p-5">
    <h4 class="flex items-center gap-2 text-sm font-semibold text-slate-800 mb-2">
      <span>⌘</span> CLI Commands
    </h4>
    <pre class="bg-slate-50 p-4 rounded-lg text-sm text-slate-600 leading-relaxed overflow-x-auto font-mono">waypoint profile show                                    # View profile
waypoint profile set --name "Jane Doe" --title "SWE"     # Update fields
waypoint profile set --skills '["Go","React","AWS"]'     # JSON arrays
waypoint profile set --greeting-style casual             # Email style</pre>
  </div>

  {#if profileData}
    <!-- Personal Info -->
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        <span>◇</span> Personal Info
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
    </div>

    <!-- Skills -->
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        <span>⚡</span> Skills
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
    </div>

    <!-- Education -->
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        <span>🎓</span> Education
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
    </div>

    <!-- Experience -->
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        <span>💼</span> Experience
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
    </div>

    <!-- Email Preferences -->
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        <span>✉</span> Email Preferences
      </h3>
      <div>
        <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Sign-Off</label>
        <div class="text-sm text-slate-700">{profileData.signOff || 'Best regards'}</div>
      </div>
    </div>
  {:else if api.profile.loading}
    <p class="text-sm text-slate-400">Loading...</p>
  {:else}
    <p class="text-sm text-slate-400">Profile not loaded (server may be down).</p>
  {/if}
</div>
