<script setup lang="ts">
import { computed, ref } from 'vue'
import { nextRarity, prevRarity, state, type Card } from '../useGame'
import { TIER_COLORS } from '../tiers'
import { t } from '../i18n'

const props = withDefaults(
  defineProps<{ card: Card; drop?: boolean; count?: number; againLabel?: string }>(),
  { drop: true, count: 0, againLabel: '' },
)
defineEmits<{ close: []; arm: []; fuse: []; defuse: []; again: [] }>()

const fuseTarget = computed(() => nextRarity(props.card.rarity))
const defuseTarget = computed(() => prevRarity(props.card.rarity))
const talismanBusy = computed(() => state.value?.talismanTier !== '')
// arming only works while standing in the card's tier (mirrors the server)
const inTier = computed(() => state.value?.tier === props.card.tier)
const wrongTierHint = computed(() =>
  t('talismanWrongTier').replace('{tier}', t('tier')[props.card.tier]),
)

const el = ref<HTMLDivElement | null>(null)
const vars = ref<Record<string, string>>({})
const flipped = ref(false)

// our rarities → the reference's data-rarity values its CSS keys off
const DATA_RARITY: Record<string, string> = {
  common: 'common',
  rare: 'reverse holo',
  holo: 'rare holo',
  prismatic: 'rare rainbow',
}

const faceStyle = computed(() => ({
  borderColor: TIER_COLORS[props.card.tier],
  // mid-tone base: the reference foils are color-dodge layers, which stay
  // black over a near-black card — they need luminance underneath to ignite
  background: `linear-gradient(165deg, ${TIER_COLORS[props.card.tier]}88, #475569 60%, ${TIER_COLORS[props.card.tier]}44)`,
  boxShadow: `0 0 50px ${TIER_COLORS[props.card.tier]}66`,
}))

// the card drop has no sound cue, so give it its own haptic flourish
navigator.vibrate?.([20, 30, 80])

function tilt(clientX: number, clientY: number) {
  const r = el.value!.getBoundingClientRect()
  const px = (clientX - r.left) / r.width
  const py = (clientY - r.top) / r.height
  // the 180° flip mirrors both tilt axes, so invert to keep the card
  // leaning toward the pointer
  const s = flipped.value ? -1 : 1
  vars.value = {
    '--rx': `${(px - 0.5) * 24 * s}deg`,
    '--ry': `${(0.5 - py) * 24 * s}deg`,
    // the reference's pointer/background spring vars, same names and ranges
    '--pointer-x': `${px * 100}%`,
    '--pointer-y': `${py * 100}%`,
    '--pointer-from-left': `${px}`,
    '--pointer-from-top': `${py}`,
    '--pointer-from-center': `${Math.min(1, Math.hypot(px - 0.5, py - 0.5) * 2)}`,
    '--background-x': `${37 + px * 26}%`,
    '--background-y': `${33 + py * 34}%`,
    '--card-opacity': '1',
  }
}

function onMove(e: MouseEvent) {
  tilt(e.clientX, e.clientY)
}

function onTouch(e: TouchEvent) {
  tilt(e.touches[0].clientX, e.touches[0].clientY)
}

// flip on tap/click via pointer events: a native click never fires on touch
// here (touchstart.prevent cancels its synthesis) and can be swallowed by
// tiny drags while tilting
let down = { x: 0, y: 0, t: 0 }
function onPointerDown(e: PointerEvent) {
  down = { x: e.clientX, y: e.clientY, t: e.timeStamp }
}
function onPointerUp(e: PointerEvent) {
  if (Math.hypot(e.clientX - down.x, e.clientY - down.y) < 10 && e.timeStamp - down.t < 500) {
    flipped.value = !flipped.value
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex touch-none select-none flex-col items-center justify-center gap-6 overscroll-contain bg-black/80 backdrop-blur-sm"
  >
    <p v-if="drop" class="text-xl font-black tracking-widest text-yellow-300">✨ {{ t('cardDrop') }}</p>
    <!-- rect is measured on this untransformed wrapper: measuring the tilted
         element itself feeds its own rotation back into the pointer math.
         pointer handlers live here too — on the tilted card they flicker at
         the edges as the rotation moves the card out from under the cursor -->
    <div
      ref="el"
      class="flip-scene"
      :class="{ active: flipped }"
      @mousemove="onMove"
      @mouseleave="vars = {}"
      @touchstart.prevent="onTouch"
      @touchmove.prevent="onTouch"
      @touchend="vars = {}"
      @pointerdown="onPointerDown"
      @pointerup="onPointerUp"
    >
      <div class="flipper card-in" :class="{ flipped }">
        <div
          class="card card-tilt h-80 w-56"
          :data-rarity="DATA_RARITY[card.rarity]"
          :style="vars"
        >
          <div
            class="card-face flex flex-col items-center justify-between rounded-2xl border-2 p-5"
            :style="faceStyle"
          >
            <span class="self-end rounded-full bg-black/40 px-2 py-0.5 text-[10px] font-bold uppercase tracking-widest text-slate-200">
              {{ t('rarity')[card.rarity] }}
            </span>
            <!-- star stays plain yellow: rarity reads from the card's foil, not the art -->
            <img src="/star.png" class="h-24 w-24 drop-shadow-[0_0_20px_#facc15]" alt="" />
            <div class="text-center">
              <p class="text-lg font-black uppercase tracking-widest" :style="{ color: TIER_COLORS[card.tier] }">
                {{ t('tier')[card.tier] }}
              </p>
              <p class="text-[10px] uppercase tracking-[0.3em] text-slate-400">the button</p>
            </div>
            <div class="card__shine"></div>
            <div class="card__glare"></div>
          </div>
          <div
            class="card-face card-back flex flex-col items-center justify-center gap-4 rounded-2xl border-2 p-5"
            :style="faceStyle"
          >
            <p class="text-4xl">🃏</p>
            <p class="text-center text-sm font-bold leading-relaxed text-slate-100">
              {{ t('talEffect')[card.rarity] }}
            </p>
            <p class="text-[10px] uppercase tracking-[0.3em] text-slate-400">the button</p>
          </div>
        </div>
      </div>
    </div>
    <div v-if="!drop" class="flex w-72 flex-col gap-2">
      <button
        class="rounded-lg bg-amber-400 py-2 text-xs font-bold text-slate-900 hover:bg-amber-300 disabled:opacity-40"
        :disabled="count < 1 || talismanBusy || !inTier"
        @click="$emit('arm')"
      >
        {{ t('talismanUse') }}
      </button>
      <div class="flex gap-2">
        <button
          v-if="fuseTarget"
          class="flex-1 whitespace-nowrap rounded-lg border border-fuchsia-400/60 py-2 text-xs font-bold text-fuchsia-300 hover:bg-fuchsia-500/20 disabled:opacity-40"
          :disabled="count < 3"
          @click="$emit('fuse')"
        >
          {{ t('fuse') }} ×3 → {{ t('rarity')[fuseTarget] }}
        </button>
        <button
          v-if="defuseTarget"
          class="flex-1 whitespace-nowrap rounded-lg border border-slate-500/60 py-2 text-xs font-bold text-slate-400 hover:bg-slate-700/40 disabled:opacity-40"
          :disabled="count < 1"
          @click="$emit('defuse')"
        >
          {{ t('defuse') }} → {{ t('rarity')[defuseTarget] }} ×2
        </button>
      </div>
      <p v-if="!inTier" class="text-center text-[10px] text-rose-400/80">{{ wrongTierHint }}</p>
      <p v-else-if="talismanBusy" class="text-center text-[10px] text-slate-500">{{ t('talismanArmedHint') }}</p>
    </div>

    <div class="flex gap-3">
      <button
        v-if="againLabel"
        class="rounded-full border border-amber-400/70 px-6 py-1.5 text-sm font-bold text-amber-300 hover:bg-amber-500/20"
        @click="$emit('again')"
      >
        🎁 {{ againLabel }}
      </button>
      <button
        class="rounded-full border border-slate-600 px-6 py-1.5 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        OK
      </button>
    </div>
  </div>
</template>
