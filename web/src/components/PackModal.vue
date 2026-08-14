<script setup lang="ts">
import { computed, ref } from 'vue'
import { state, type Card } from '../useGame'
import { cfg } from '../config'
import { t } from '../i18n'
import { play, scratchTick, vibrate, type Sound } from '../audio'
import { burst, confetti, reducedMotion } from '../particles'
import { COIN_COLORS } from '../useCoinCounter'
import { tiltVars } from '../cardTilt'
import CardFace from './CardFace.vue'

const props = defineProps<{ cards: Card[] }>()
const emit = defineEmits<{ close: []; again: [] }>()

const canAgain = computed(() => (state.value?.coins ?? 0) >= cfg().pack.price)

// sealed (tap or zipper-drag) -> tearing (burst animation) -> open (face-down cards)
const stage = ref<'sealed' | 'tearing' | 'open'>('sealed')
const flipped = ref(props.cards.map(() => false))
const allFlipped = computed(() => flipped.value.every(Boolean))

// the tear theatre is scaled to the best card in the pack, so a prismatic
// still announces itself even when it comes out last
const BANDS = ['common', 'rare', 'holo', 'prismatic']
const best = computed(() =>
  props.cards.reduce((a, c) => (BANDS.indexOf(c.rarity) > BANDS.indexOf(a) ? c.rarity : a), 'common'),
)

const RARITY_FX: Record<
  string,
  { colors: string[]; count: number; sound: 'success_silver' | 'success_gold' | 'success_diamond' | 'win'; glow: string }
> = {
  common: { colors: COIN_COLORS, count: 40, sound: 'success_silver', glow: '#94a3b855' },
  rare: { colors: ['#7dd3fc', '#38bdf8', '#0ea5e9', '#ffffff'], count: 70, sound: 'success_gold', glow: '#38bdf866' },
  holo: { colors: ['#f0abfc', '#fcd34d', '#22d3ee', '#a78bfa', '#ffffff'], count: 110, sound: 'success_diamond', glow: '#a78bfa77' },
  prismatic: {
    colors: ['#ff0084', '#fcff00', '#00fff0', '#7c00ff', '#ffffff'],
    count: 160,
    sound: 'win',
    glow: 'conic-gradient(#ff0084, #fcff00, #00fff0, #7c00ff, #ff0084)',
  },
}
const TEAR_MS = 450

// cards keep native size inside a scaled flip-scene so the foil CSS is
// untouched; the wrapper takes the scaled footprint so flex lays out right.
// The scale fits the row to the viewport: full size wherever it fits (PC),
// shrinking only as far as a narrow phone forces it. 240 = card + gap.
const k = computed(() => Math.min(1, (window.innerWidth - 48) / (props.cards.length * 240)))

// per-card poke-holo tilt, driven from the untransformed sized wrapper
const tilt = ref<Record<string, string>[]>(props.cards.map(() => ({})))
function onCardMove(i: number, x: number, y: number, e: Event) {
  tilt.value[i] = tiltVars(x, y, (e.currentTarget as HTMLElement).getBoundingClientRect())
}

function tear() {
  if (stage.value !== 'sealed') return
  stage.value = 'tearing'
  const fx = RARITY_FX[best.value]
  burst(window.innerWidth / 2, window.innerHeight / 2, fx.colors, fx.count)
  play(fx.sound)
  vibrate(best.value === 'prismatic' ? [40, 30, 80, 30, 120] : [20, 25, 45])
  if (best.value === 'prismatic') confetti()
  setTimeout(() => (stage.value = 'open'), reducedMotion.matches ? 0 : TEAR_MS)
}

// zipper: drag along the tear line erodes the foil strip; a plain tap (or a
// release past halfway) commits the tear, a short release snaps the strip back
const ZIPPER_PX = 180
const progress = ref(0)
const dragging = ref(false)
let down = { x: 0, t: 0 }

function onDown(e: PointerEvent) {
  if (stage.value !== 'sealed') return
  down = { x: e.clientX, t: e.timeStamp }
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onMove(e: PointerEvent) {
  if (!dragging.value) return
  const p = Math.min(1, Math.max(0, (e.clientX - down.x) / ZIPPER_PX))
  if (p > progress.value) {
    scratchTick()
    if (Math.floor(p * 10) > Math.floor(progress.value * 10)) navigator.vibrate?.(4)
  }
  progress.value = p
  if (p >= 0.95) {
    dragging.value = false
    tear()
  }
}

function onUp(e: PointerEvent) {
  if (!dragging.value) return
  dragging.value = false
  const tap = Math.abs(e.clientX - down.x) < 10 && e.timeStamp - down.t < 500
  if (tap || progress.value >= 0.5) tear()
  else progress.value = 0
}

// tier picks the pitch, rarity the flourish — prismatic escalates to the win jingle
function flip(i: number) {
  if (flipped.value[i]) return
  flipped.value[i] = true
  const c = props.cards[i]
  const fx = RARITY_FX[c.rarity]
  // the physical flip first, the rarity fanfare on top of it
  play('card-flip')
  play(c.rarity === 'prismatic' ? 'win' : (`success_${c.tier}` as Sound))
  burst(window.innerWidth / 2, window.innerHeight / 2, fx.colors, Math.round(fx.count / 2))
  vibrate(c.rarity === 'prismatic' ? [40, 30, 80] : 20)
  if (c.rarity === 'prismatic') confetti()
}

function flipAll() {
  const step = reducedMotion.matches ? 0 : 250
  props.cards
    .map((_, i) => i)
    .filter((i) => !flipped.value[i])
    .forEach((i, j) => setTimeout(() => flip(i), j * step))
}
</script>

<template>
  <div
    v-if="stage === 'open'"
    class="fixed inset-0 z-50 flex touch-none select-none flex-col items-center justify-center gap-6 bg-black/80 backdrop-blur-sm"
  >
    <div
      class="pack-glow pointer-events-none absolute h-80 w-80 rounded-full blur-3xl"
      :style="{ background: RARITY_FX[best].glow }"
    ></div>
    <div class="relative flex items-center justify-center gap-3">
      <div
        v-for="(c, i) in cards"
        :key="i"
        class="card-in [animation-fill-mode:backwards]"
        :style="{ width: 224 * k + 'px', height: 320 * k + 'px', animationDelay: i * 100 + 'ms' }"
        @mousemove="onCardMove(i, $event.clientX, $event.clientY, $event)"
        @mouseleave="tilt[i] = {}"
        @touchmove="onCardMove(i, $event.touches[0].clientX, $event.touches[0].clientY, $event)"
        @touchend="tilt[i] = {}"
      >
        <div class="flip-scene h-80 w-56 origin-top-left" :style="{ transform: `scale(${k})` }">
          <div class="flip-card h-full w-full cursor-pointer" :class="{ flipped: flipped[i] }" @click="flip(i)">
            <div
              class="flip-back flex h-full w-full flex-col items-center justify-center gap-3 rounded-2xl border-2 border-amber-300/60 bg-gradient-to-b from-indigo-800 via-violet-900 to-zinc-900"
            >
              <img src="/star.png" class="h-20 w-20 drop-shadow-[0_0_25px_#facc15]" alt="" />
              <p class="text-lg font-black tracking-widest text-amber-300">THE BUTTON</p>
            </div>
            <div class="flip-front absolute inset-0">
              <CardFace :card="c" class="card-tilt" :style="tilt[i]" />
            </div>
          </div>
        </div>
      </div>
    </div>
    <button
      v-if="!allFlipped"
      data-space
      class="rounded-full border border-amber-400/70 px-6 py-1.5 text-sm font-bold text-amber-300 hover:bg-amber-500/20"
      @click="flipAll"
    >
      🃏 {{ t('packFlipAll') }}
    </button>
    <div v-else class="flex gap-3">
      <button
        v-if="canAgain"
        class="rounded-full border border-amber-400/70 px-6 py-1.5 text-sm font-bold text-amber-300 hover:bg-amber-500/20"
        @click="emit('again')"
      >
        🎁 {{ t('openAnother') }} (💰{{ cfg().pack.price }})
      </button>
      <button
        data-space
        class="rounded-full border border-zinc-600 px-6 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800"
        @click="emit('close')"
      >
        OK
      </button>
    </div>
  </div>
  <div
    v-else
    class="fixed inset-0 z-50 flex flex-col items-center justify-center gap-6 bg-black/80 backdrop-blur-sm"
  >
    <p class="text-sm tracking-widest text-zinc-400">{{ t('packTear') }}</p>
    <!-- tear() self-guards on stage, so the synthetic data-space click can't
         double-fire after a real pointer tap already tore the pack -->
    <button
      data-space
      class="pack-seal relative flex h-72 w-52 touch-none select-none flex-col items-center justify-center gap-3 rounded-2xl border-2 border-amber-300/60 bg-gradient-to-b from-indigo-800 via-violet-900 to-zinc-900"
      :class="stage === 'tearing' ? 'pack-tear' : progress ? '' : 'pack-idle cursor-pointer'"
      @click="tear"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
    >
      <img src="/star.png" class="pointer-events-none h-20 w-20 drop-shadow-[0_0_25px_#facc15]" alt="" />
      <p class="text-xl font-black tracking-widest text-amber-300">THE BUTTON</p>
      <p class="text-[10px] uppercase tracking-[0.3em] text-indigo-200">{{ t('packName') }}</p>
      <span class="absolute inset-x-4 top-3 border-t-2 border-dashed border-amber-200/40"></span>
      <span
        class="pointer-events-none absolute inset-x-0 top-0 h-7 rounded-t-2xl border-b-2 border-dashed border-amber-200/60 bg-gradient-to-b from-indigo-700 to-violet-900"
        :style="{ clipPath: `inset(0 0 0 ${progress * 100}%)`, transition: dragging ? 'none' : 'clip-path .25s' }"
      ></span>
    </button>
  </div>
</template>

<style scoped>
@keyframes pack-idle {
  0%, 100% { transform: rotate(-1.5deg) scale(1); box-shadow: 0 0 30px #a78bfa66; }
  50% { transform: rotate(1.5deg) scale(1.03); box-shadow: 0 0 55px #a78bfaaa; }
}
.pack-idle {
  animation: pack-idle 1.6s ease-in-out infinite;
}
@keyframes pack-tear {
  0% { transform: scale(1); opacity: 1; filter: brightness(1); }
  40% { transform: scale(1.18); filter: brightness(2.2); }
  100% { transform: scale(1.6); opacity: 0; filter: brightness(3); }
}
.pack-tear {
  animation: pack-tear 0.45s ease-out forwards;
  pointer-events: none;
}
.flip-card {
  position: relative;
  transform-style: preserve-3d;
  transition: transform 0.6s cubic-bezier(0.2, 1.4, 0.4, 1);
}
.flip-card.flipped {
  transform: rotateY(180deg);
}
.flip-back,
.flip-front {
  backface-visibility: hidden;
}
.flip-front {
  transform: rotateY(180deg);
}
@keyframes pack-glow {
  0%, 100% { opacity: 0.45; transform: scale(1); }
  50% { opacity: 0.8; transform: scale(1.12); }
}
.pack-glow {
  animation: pack-glow 2s ease-in-out infinite;
}
</style>
