const files = [
  'click', 'fail', 'hover', 'mouseover', 'switch', 'win',
  'success_unrank', 'success_bronze', 'success_silver',
  'success_gold', 'success_platinum', 'success_diamond',
] as const

export type Sound = (typeof files)[number]

const cache = new Map<Sound, HTMLAudioElement>()
for (const f of files) {
  const a = new Audio(`/${f}.wav`)
  a.preload = 'auto'
  cache.set(f, a)
}

export const soundCount = files.length

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

// synthesized coin-on-latex scratch: a short bandpassed noise burst per stroke.
// no CC0 scratch-card sample with a scriptable download exists, and synthesis
// varies naturally with every stroke anyway
let scratchCtx: AudioContext | null = null
let lastTick = 0
const SCRATCH_THROTTLE_MS = 70

export function scratchTick() {
  const now = performance.now()
  if (now - lastTick < SCRATCH_THROTTLE_MS) return
  lastTick = now
  scratchCtx ??= new AudioContext()
  const ctx = scratchCtx
  const len = Math.floor(ctx.sampleRate * 0.06)
  const buf = ctx.createBuffer(1, len, ctx.sampleRate)
  const data = buf.getChannelData(0)
  for (let i = 0; i < len; i++) data[i] = Math.random() * 2 - 1
  const src = ctx.createBufferSource()
  src.buffer = buf
  const bp = ctx.createBiquadFilter()
  bp.type = 'bandpass'
  bp.frequency.value = 2500 + (Math.random() - 0.5) * 1000
  bp.Q.value = 1.2
  const gain = ctx.createGain()
  gain.gain.setValueAtTime(0.25, ctx.currentTime)
  gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.06)
  src.connect(bp).connect(gain).connect(ctx.destination)
  src.start()
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
}

export function play(name: Sound) {
  const pattern = buzz[name] ?? (name.startsWith('success_') ? 30 : undefined)
  if (pattern) navigator.vibrate?.(pattern)
  const base = cache.get(name)
  if (!base) return
  // clone so rapid replays overlap instead of cutting off
  const a = base.cloneNode() as HTMLAudioElement
  a.volume = 0.6
  a.play().catch(() => {}) // autoplay restrictions before first interaction
}
