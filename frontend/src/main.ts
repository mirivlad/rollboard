import './styles/tokens.css';
import App from './App.svelte';
import { mount } from 'svelte';
import { themeStore } from './lib/theme.svelte';

// Applied before the first paint so the interface never flashes the wrong theme.
themeStore.init();

const app = mount(App, {
  target: document.getElementById('app')!,
});

export default app;
