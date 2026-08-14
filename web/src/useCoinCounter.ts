import { onUnmounted, ref, watch, type Ref } from 'vue'

const COIN_TWEEN_MS = 700

/** A display value that rolls toward the source number odometer-style
 * whenever it changes, instead of jumping. */
export function useCoinCounter(source: () => number): Ref<number> {
  const shown = ref(source())
  let raf = 0
  watch(source, (to, from) => {
    cancelAnimationFrame(raf)
    const start = performance.now()
    const animate = (now: number) => {
      const p = Math.min((now - start) / COIN_TWEEN_MS, 1)
      const ease = 1 - Math.pow(1 - p, 3)
      shown.value = Math.round(from + (to - from) * ease)
      if (p < 1) raf = requestAnimationFrame(animate)
    }
    raf = requestAnimationFrame(animate)
  })
  onUnmounted(() => cancelAnimationFrame(raf))
  return shown
}

/** Gold palette for coin particle bursts. */
export const COIN_COLORS = ['#facc15', '#fbbf24', '#f59e0b', '#fde68a']

/** Compact coin display: up to 4 digits stay raw, then 12.3k / 4.5m / 1.2b —
 * one decimal while it matters, dropped once the number is 3 digits wide. */
export function fmtCoins(n: number): string {
  if (n < 10_000) return String(n)
  for (const [div, suffix] of [
    [1e9, 'b'],
    [1e6, 'm'],
    [1e3, 'k'],
  ] as const) {
    if (n >= div) {
      const v = n / div
      return (v >= 100 ? String(Math.round(v)) : v.toFixed(1).replace(/\.0$/, '')) + suffix
    }
  }
  return String(n)
}
