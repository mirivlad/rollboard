<script lang="ts">
  import { ApiError, api } from '../lib/api';
  import { i18n } from '../lib/i18n.svelte';

  type Props = {
    value: string;
    onChange: (url: string) => void;
  };

  let { value, onChange }: Props = $props();
  let t = $derived(i18n.t);
  let busy = $state(false);
  let error = $state('');
  let input: HTMLInputElement | undefined = $state();

  async function upload(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0];
    if (!file) return;
    busy = true;
    error = '';
    try {
      const { url } = await api.uploadImage(file);
      onChange(url);
    } catch (cause) {
      // The server decides what is too large or unsupported; the interface
      // only translates its answer.
      if (cause instanceof ApiError && cause.code === 'UPLOAD_TOO_LARGE') error = t('editor.imageTooLarge');
      else if (cause instanceof ApiError && cause.code === 'UNSUPPORTED_IMAGE') error = t('editor.imageUnsupported');
      else error = t('app.genericError');
    } finally {
      busy = false;
      if (input) input.value = '';
    }
  }
</script>

<div class="image-field">
  <label>
    {t('editor.image')}
    <input value={value ?? ''} placeholder={t('inspector.imageUrlPlaceholder')} oninput={(e) => onChange((e.target as HTMLInputElement).value)} />
  </label>

  <div class="row">
    <input
      bind:this={input}
      class="file"
      type="file"
      accept="image/png,image/jpeg,image/gif,image/webp"
      onchange={upload}
      disabled={busy}
      aria-label={t('editor.uploadImage')}
    />
    {#if busy}<span class="busy">{t('editor.uploading')}</span>{/if}
  </div>

  {#if value}
    <img class="preview" src={value} alt="" />
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
</div>

<style>
  .image-field {
    display: grid;
    gap: var(--space-1);
  }
  label {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  input {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
  }
  .row {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .file {
    flex: 1;
    min-width: 0;
    font-size: var(--text-xs);
  }
  .busy {
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .preview {
    width: 100%;
    max-height: 96px;
    object-fit: contain;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: var(--text-xs);
  }
</style>
