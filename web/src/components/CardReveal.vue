<script setup lang="ts">
import { computed, ref } from 'vue'
import { nextRarity, prevRarity, state, type Card } from '../useGame'
import { t } from '../i18n'
import CardFace from './CardFace.vue'

const props = withDefaults(
  defineProps<{ card: Card; drop?: boolean; count?: number; againLabel?: string }>(),
  { drop: true, count: 0, againLabel: '' },
)
defineEmits<{ close: []; arm: []; fuse: []; defuse: []; again: [] }>()

const fuseTarget = computed(() => nextRarity(props.card.rarity))
const defuseTarget = computed(() => prevRarity(props.card.rarity))
const talismanBusy = computed(() => state.value?.talismanTier !== '')

const el = ref<HTMLDivElement | null>(null)
const vars = ref<Record<string, string>>({})
const active = ref(false)

// the card drop has no sound cue, so give it its own haptic flourish
navigator.vibrate?.([20, 30, 80])

function tilt(clientX: number, clientY: number) {
  const r = el.value!.getBoundingClientRect()
  const px = (clientX - r.left) / r.width
  const py = (clientY - r.top) / r.height
  vars.value = {
    '--rx': `${(px - 0.5) * 24}deg`,
    '--ry': `${(0.5 - py) * 24}deg`,
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

// zoom pop on tap/click via pointer events: a native click never fires on
// touch here (touchstart.prevent cancels its synthesis) and can be swallowed
// by tiny drags while tilting
let down = { x: 0, y: 0, t: 0 }
function onPointerDown(e: PointerEvent) {
  down = { x: e.clientX, y: e.clientY, t: e.timeStamp }
}
function onPointerUp(e: PointerEvent) {
  if (Math.hypot(e.clientX - down.x, e.clientY - down.y) < 10 && e.timeStamp - down.t < 500) {
    active.value = !active.value
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex touch-none select-none flex-col items-center justify-center gap-6 overscroll-contain bg-black/80 backdrop-blur-sm"
    @click.self="$emit('close')"
  >
    <p v-if="drop" class="text-xl font-black tracking-widest text-yellow-300">✨ {{ t('cardDrop') }}</p>
    <!-- rect is measured on this untransformed wrapper: measuring the tilted
         element itself feeds its own rotation back into the pointer math.
         pointer handlers live here too — on the tilted card they flicker at
         the edges as the rotation moves the card out from under the cursor -->
    <div
      ref="el"
      class="flip-scene"
      :class="{ active }"
      @mousemove="onMove"
      @mouseleave="vars = {}"
      @touchstart.prevent="onTouch"
      @touchmove.prevent="onTouch"
      @touchend="vars = {}"
      @pointerdown="onPointerDown"
      @pointerup="onPointerUp"
    >
      <CardFace :card="card" class="card-tilt card-in" :style="vars" />
    </div>
    <div v-if="!drop" class="flex w-72 flex-col gap-2">
      <button
        class="rounded-lg bg-amber-400 py-2 text-xs font-bold text-black hover:bg-amber-300 disabled:opacity-40"
        :disabled="count < 1 || talismanBusy"
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
          class="flex-1 whitespace-nowrap rounded-lg border border-zinc-500/60 py-2 text-xs font-bold text-zinc-400 hover:bg-zinc-700/40 disabled:opacity-40"
          :disabled="count < 1"
          @click="$emit('defuse')"
        >
          {{ t('defuse') }} → {{ t('rarity')[defuseTarget] }} ×2
        </button>
      </div>
      <p class="text-center text-[10px] text-zinc-500">
        {{ talismanBusy ? t('talismanArmedHint') : t('talismanNextClick') }}
      </p>
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
        class="rounded-full border border-zinc-600 px-6 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800"
        @click="$emit('close')"
      >
        OK
      </button>
    </div>
  </div>
</template>
