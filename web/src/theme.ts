import { ref } from 'vue'

type Theme = 'dark' | 'light'

// index.html resolves and applies the theme before first paint; this ref
// mirrors it for the toggle button
export const theme = ref<Theme>(
  (document.documentElement.dataset.theme as Theme) ?? 'dark',
)

const THEME_COLOR: Record<Theme, string> = { dark: '#0f172a', light: '#eef2f7' }

function apply(t: Theme) {
  theme.value = t
  document.documentElement.dataset.theme = t
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', THEME_COLOR[t])
}

apply(theme.value)

// follow the OS live, but only while the user hasn't made an explicit choice
matchMedia('(prefers-color-scheme: light)').addEventListener('change', (e) => {
  if (!localStorage.getItem('theme')) apply(e.matches ? 'light' : 'dark')
})

export function toggleTheme() {
  const t: Theme = theme.value === 'dark' ? 'light' : 'dark'
  localStorage.setItem('theme', t)
  // crossfade the whole page where the View Transitions API exists
  if (document.startViewTransition) document.startViewTransition(() => apply(t))
  else apply(t)
}
