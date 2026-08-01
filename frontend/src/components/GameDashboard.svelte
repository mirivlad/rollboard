<script lang="ts">
  export type Template = 'blank' | 'mini-monopoly' | 'dungeon-race';

  type Props = {
    displayName: string;
    onCreate: (template: Template, advanced: boolean) => void | Promise<void>;
    busy?: boolean;
  };

  let { displayName, onCreate, busy = false }: Props = $props();
  let advanced = $state(false);
</script>

<section class="dashboard">
  <header>
    <div>
      <p class="eyebrow">AUTHOR WORKSPACE</p>
      <h1>Your games</h1>
      <p>Welcome, {displayName}. Drafts are private until you publish a version.</p>
    </div>
    <label class="advanced"><input type="checkbox" bind:checked={advanced} /> Advanced studio</label>
  </header>

  <section class="create" aria-label="Create a game">
    <h2>Start a game</h2>
    <div class="templates">
      <button onclick={() => onCreate('blank', advanced)} disabled={busy}>
        <strong>Blank board</strong><span>Build rules and board from scratch</span>
      </button>
      <button onclick={() => onCreate('mini-monopoly', advanced)} disabled={busy}>
        <strong>Mini-Monopoly</strong><span>Property, rent and choices</span>
      </button>
      <button onclick={() => onCreate('dungeon-race', advanced)} disabled={busy}>
        <strong>Dungeon Race</strong><span>Fast race with resources</span>
      </button>
    </div>
  </section>

  <section class="empty" aria-label="Draft list">
    <h2>Private drafts</h2>
    <p>No drafts yet. Pick a template to create your first game.</p>
  </section>
</section>

<style>
  .dashboard { width: min(1120px, 100%); margin: 0 auto; color: #eef3ff; }
  header { display: flex; justify-content: space-between; gap: 1rem; align-items: start; margin-bottom: 2rem; }
  .eyebrow { margin: 0; color: #75d4ff; letter-spacing: .12em; font-size: .72rem; font-weight: 800; }
  h1 { margin: .35rem 0; font-size: clamp(2rem, 5vw, 3rem); }
  header p:not(.eyebrow) { margin: 0; color: #aab6d3; }
  .advanced { display: flex; gap: .5rem; align-items: center; padding: .65rem .8rem; border: 1px solid #30415f; border-radius: 8px; color: #cad5ed; white-space: nowrap; }
  h2 { margin: 0 0 1rem; font-size: 1.1rem; }
  .templates { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1rem; }
  .templates button { min-height: 142px; display: grid; align-content: center; gap: .55rem; padding: 1.1rem; border: 1px solid #2d4165; border-radius: 14px; background: linear-gradient(140deg, #172a49, #101a30); color: #f1f6ff; text-align: left; cursor: pointer; }
  .templates button:hover { border-color: #68cdf5; transform: translateY(-1px); }
  .templates span { color: #a9b9d7; line-height: 1.4; }
  .empty { margin-top: 2rem; padding: 1.5rem; border: 1px dashed #34476b; border-radius: 14px; color: #aebbd4; }
  .empty p { margin-bottom: 0; }
  @media (max-width: 720px) { header { display: grid; } .templates { grid-template-columns: 1fr; } }
</style>
