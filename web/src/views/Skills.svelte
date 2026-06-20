<script>
  import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import * as api from '../stores/api.svelte.js';

  onMount(() => { setPage({ title: 'AI Integration' }); });

  const agents = [
    { id: 'pi.dev', name: 'Pi', dir: '.pi/skills/waypoint' },
    { id: 'claude-code', name: 'Claude Code', dir: '.claude/skills/waypoint' },
    { id: 'codex', name: 'Codex', dir: '.codex/skills/waypoint' },
    { id: 'opencode', name: 'OpenCode', dir: '.opencode/skills/waypoint' },
  ];

  import { skillMeta, skillLabel } from '../stores/skillMeta.js';
  import { iconSvg } from '../lib/icons.js';

  const skills = Object.entries(skillMeta).map(([id, meta]) => ({
    id,
    name: `${skillLabel(id)} Generator`,
    desc: meta.tags.join(', '),
    iconName: id,
    tags: meta.tags,
  }));

  let selectedAgent = $state('pi.dev');
  let copied = $state(false);

  function copyInstallCmd() {
    navigator.clipboard.writeText(`waypoint skills install --agent ${selectedAgent}`);
    copied = true;
    setTimeout(() => copied = false, 1500);
  }
</script>

<div class="space-y-6">
  <p class="text-sm text-slate-400">
    Connect your AI coding agent to Waypoint — it learns the CLI commands and can generate job-search content on demand.
  </p>

  <!-- Install section -->
  <div class="bg-white rounded-xl border border-slate-200 p-5">
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-1">{@html iconSvg('bot', 18)} Install the Waypoint skill</h3>
    <p class="text-sm text-slate-400 mb-3">Run this in your project directory. The skill teaches your agent how to use <code class="bg-slate-100 px-1 rounded text-xs">waypoint</code>.</p>
    <div class="flex items-center gap-2 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2.5 font-mono text-sm mb-3">
      <code class="flex-1 text-slate-700">waypoint skills install --agent {selectedAgent}</code>
      <button
        class="px-2.5 py-1 rounded text-xs font-medium cursor-pointer transition-colors {copied ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600 hover:bg-slate-300'}"
        onclick={copyInstallCmd}
      >{copied ? '✓ Copied' : 'Copy'}</button>
    </div>
    <div class="flex flex-wrap items-center gap-1.5">
      {#each agents as agent}
        <button
          class="rounded-full px-3 py-1 text-xs cursor-pointer transition-colors {selectedAgent === agent.id ? 'bg-slate-800 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'}"
          onclick={() => selectedAgent = agent.id}
        >{agent.name}</button>
      {/each}
      <span class="text-xs text-slate-400 ml-1">→ installs to <code class="bg-slate-100 px-1 rounded">{agents.find(a => a.id === selectedAgent)?.dir}/</code></span>
    </div>
  </div>

  <!-- Skills list -->
  <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800">{@html iconSvg('zap', 18)} Available skills</h3>
  {#each skills as skill}
    <div class="bg-white rounded-xl border border-slate-200 p-5 hover:border-slate-400 hover:shadow-sm transition-all">
      <h3 class="text-base font-semibold text-slate-800 mb-1">{@html iconSvg(skill.iconName, 18)} {skill.name}</h3>
      <p class="text-sm text-slate-500 mb-3">{skill.desc}</p>
      <div class="flex flex-wrap gap-1.5">
        {#each skill.tags as tag}
          <span class="bg-slate-100 border border-slate-200 text-slate-500 rounded-full px-2.5 py-0.5 text-[11px]">{tag}</span>
        {/each}
      </div>
    </div>
  {/each}
</div>
