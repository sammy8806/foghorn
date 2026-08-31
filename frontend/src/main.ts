import './style.css'
import App from './App.svelte'
import { seedPlatform } from './stores/platform'

// Runs before the first paint so the macOS traffic-light inset and drag region
// are already in place; syncPlatform() later replaces the guess with the value
// the Wails runtime reports.
seedPlatform()

const app = new App({
  target: document.getElementById('app')
})

export default app
