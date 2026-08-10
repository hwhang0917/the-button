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

export function play(name: Sound) {
  const base = cache.get(name)
  if (!base) return
  // clone so rapid replays overlap instead of cutting off
  const a = base.cloneNode() as HTMLAudioElement
  a.volume = 0.6
  a.play().catch(() => {}) // autoplay restrictions before first interaction
}
