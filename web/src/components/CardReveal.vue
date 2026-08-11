<script setup lang="ts">
import { computed, ref } from 'vue'
import { nextRarity, prevRarity, state, type Card } from '../useGame'
import { TIER_COLORS } from '../tiers'
import { t } from '../i18n'

const props = withDefaults(defineProps<{ card: Card; drop?: boolean; count?: number }>(), {
  drop: true,
  count: 0,
})
defineEmits<{ close: []; arm: []; fuse: []; defuse: [] }>()

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

// the card drop has no sound cue, so give it its own haptic flourish
navigator.vibrate?.([20, 30, 80])

function tilt(clientX: number, clientY: number) {
  const r = el.value!.getBoundingClientRect()
  const px = (clientX - r.left) / r.width
  const py = (clientY - r.top) / r.height
  vars.value = {
    '--rx': `${(px - 0.5) * 24}deg`,
    '--ry': `${(0.5 - py) * 24}deg`,
    '--gx': `${px * 100}%`,
    '--gy': `${py * 100}%`,
  }
}

function onMove(e: MouseEvent) {
  tilt(e.clientX, e.clientY)
}

function onTouch(e: TouchEvent) {
  tilt(e.touches[0].clientX, e.touches[0].clientY)
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex touch-none flex-col items-center justify-center gap-6 overscroll-contain bg-black/80 backdrop-blur-sm"
  >
    <p v-if="drop" class="text-xl font-black tracking-widest text-yellow-300">✨ {{ t('cardDrop') }}</p>
    <div
      ref="el"
      class="holo-card card-in flex h-80 w-56 flex-col items-center justify-between rounded-2xl border-2 p-5"
      :class="`rarity-${card.rarity}`"
      :style="{
        ...vars,
        borderColor: TIER_COLORS[card.tier],
        background: `linear-gradient(165deg, ${TIER_COLORS[card.tier]}55, #0b1120 60%, ${TIER_COLORS[card.tier]}22)`,
        boxShadow: `0 0 50px ${TIER_COLORS[card.tier]}66`,
      }"
      @mousemove="onMove"
      @mouseleave="vars = {}"
      @touchstart.prevent="onTouch"
      @touchmove.prevent="onTouch"
      @touchend="vars = {}"
    >
      <span class="self-end rounded-full bg-black/40 px-2 py-0.5 text-[10px] font-bold uppercase tracking-widest text-slate-200">
        {{ t('rarity')[card.rarity] }}
      </span>
      <img
        src="/star.png"
        class="h-24 w-24"
        :class="
          card.rarity === 'rare'
            ? 'star-r1'
            : card.rarity === 'holo'
              ? 'star-r2'
              : card.rarity === 'prismatic'
                ? 'star-r3'
                : 'drop-shadow-[0_0_20px_#facc15]'
        "
        alt=""
      />
      <div class="text-center">
        <p class="text-lg font-black uppercase tracking-widest" :style="{ color: TIER_COLORS[card.tier] }">
          {{ t('tier')[card.tier] }}
        </p>
        <p class="text-[10px] uppercase tracking-[0.3em] text-slate-400">the button</p>
      </div>
    </div>
    <div v-if="!drop" class="flex w-64 flex-col gap-2">
      <p class="text-center text-xs text-slate-400">
        🃏 {{ t('talEffect')[card.rarity] }}
      </p>
      <div class="flex gap-2">
        <button
          class="flex-1 rounded-lg bg-amber-400 py-2 text-xs font-bold text-slate-900 hover:bg-amber-300 disabled:opacity-40"
          :disabled="count < 1 || talismanBusy || !inTier"
          @click="$emit('arm')"
        >
          {{ t('talismanUse') }}
        </button>
        <button
          v-if="fuseTarget"
          class="flex-1 rounded-lg border border-fuchsia-400/60 py-2 text-xs font-bold text-fuchsia-300 hover:bg-fuchsia-500/20 disabled:opacity-40"
          :disabled="count < 3"
          @click="$emit('fuse')"
        >
          {{ t('fuse') }} ×3 → {{ t('rarity')[fuseTarget] }}
        </button>
        <button
          v-if="defuseTarget"
          class="flex-1 rounded-lg border border-slate-500/60 py-2 text-xs font-bold text-slate-400 hover:bg-slate-700/40 disabled:opacity-40"
          :disabled="count < 1"
          @click="$emit('defuse')"
        >
          {{ t('defuse') }} → {{ t('rarity')[defuseTarget] }} ×2
        </button>
      </div>
      <p v-if="!inTier" class="text-center text-[10px] text-rose-400/80">{{ wrongTierHint }}</p>
      <p v-else-if="talismanBusy" class="text-center text-[10px] text-slate-500">{{ t('talismanArmedHint') }}</p>
    </div>

    <button
      class="rounded-full border border-slate-600 px-6 py-1.5 text-sm text-slate-300 hover:bg-slate-800"
      @click="$emit('close')"
    >
      OK
    </button>
  </div>
</template>
