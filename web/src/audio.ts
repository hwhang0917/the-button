import { ref } from 'vue'

const files = [
  'click', 'fail', 'hover', 'mouseover', 'switch', 'win', 'prestige',
  'scratch', 'shield', 'streak-sell', 'coin-use', 'talisman',
  'success_unrank', 'success_bronze', 'success_silver',
  'success_gold', 'success_platinum', 'success_diamond',
] as const

export type Sound = (typeof files)[number]

const cache = new Map<Sound, HTMLAudioElement>()
for (const f of files) {
  const a = new Audio(`/${f}.mp3`)
  a.preload = 'auto'
  cache.set(f, a)
}

export const soundCount = files.length

// sound-only mute: haptics keep working — they are the quiet channel
export const muted = ref(localStorage.getItem('muted') === '1')
export function toggleMute() {
  muted.value = !muted.value
  localStorage.setItem('muted', muted.value ? '1' : '0')
}

/** Buffers every sound; each resolves on ready, error, or timeout — a stalled
 * download must not hold the loading screen hostage. */
export function preloadAudio(onEach: () => void, timeoutMs: number): Promise<void> {
  return Promise.all(
    [...cache.values()].map(
      (a) =>
        new Promise<void>((resolve) => {
          let settled = false
          const done = () => {
            if (settled) return
            settled = true
            onEach()
            resolve()
          }
          if (a.readyState >= HTMLMediaElement.HAVE_ENOUGH_DATA) return done()
          setTimeout(done, timeoutMs)
          a.addEventListener('canplaythrough', done, { once: true })
          a.addEventListener('error', done, { once: true })
          a.load()
        }),
    ),
  ).then(() => {})
}

// real scratch sample now; throttled so continuous strokes don't stack clones
let lastTick = 0
const SCRATCH_THROTTLE_MS = 180

export function scratchTick() {
  const now = performance.now()
  if (now - lastTick < SCRATCH_THROTTLE_MS) return
  lastTick = now
  play('scratch')
}

/** Haptic helper: no-ops where the Vibration API is missing (iOS Safari).
 * A later call replaces a running pattern, so richer patterns should be
 * fired AFTER play() to override its generic buzz. */
export function vibrate(pattern: number | number[]) {
  navigator.vibrate?.(pattern)
}

// haptics piggyback on the sound cues; navigator.vibrate is missing on iOS
// Safari, so the optional call quietly no-ops there
const buzz: Partial<Record<Sound, number | number[]>> = {
  click: 15,
  switch: 10,
  fail: [60, 40, 120],
  win: [50, 50, 50, 50, 150],
  prestige: [50, 50, 50, 50, 150],
}

// roll-result jingles are long: a fresh button press (click) or the next
// result cancels the one still playing, so spam-rolling doesn't stack fanfares
const resultSounds = new Set<Sound>(
  files.filter((f) => f === 'fail' || f === 'win' || f === 'prestige' || f.startsWith('success_')),
)
let playingResult: HTMLAudioElement | null = null

export function play(name: Sound) {
  const pattern = buzz[name] ?? (name.startsWith('success_') ? 30 : undefined)
  if (pattern) navigator.vibrate?.(pattern)
  if (muted.value) return
  const base = cache.get(name)
  if (!base) return
  // clone so rapid replays overlap instead of cutting off
  const a = base.cloneNode() as HTMLAudioElement
  a.volume = 0.6
  if (name === 'click' || resultSounds.has(name)) {
    playingResult?.pause()
    playingResult = resultSounds.has(name) ? a : null
  }
  a.play().catch(() => {}) // autoplay restrictions before first interaction
}
