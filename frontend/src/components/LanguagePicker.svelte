<script lang="ts">
  import { i18n, localeName } from '../lib/i18n.svelte';

  let t = $derived(i18n.t);
</script>

<label class="language">
  <span class="visually-hidden">{t('app.language')}</span>
  <select
    value={i18n.locale}
    onchange={(event) => i18n.setLocale((event.target as HTMLSelectElement).value)}
    aria-label={t('app.language')}
  >
    {#each i18n.available as tag (tag)}
      <option value={tag}>{localeName(tag)}</option>
    {/each}
  </select>
</label>

<style>
  .language { display: inline-flex; align-items: center; }
  select {
    padding: .4rem .5rem;
    border: 1px solid #385071;
    border-radius: 8px;
    background: #0b1224;
    color: #eff5ff;
    font: inherit;
    font-size: .85rem;
    cursor: pointer;
  }
  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    padding: 0;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
