<script setup lang="ts">
import { computed } from 'vue'
import { cfg } from '../config'
import { t } from '../i18n'

const props = withDefaults(defineProps<{ stars: number; prestige?: number; max: number }>(), {
  prestige: 0,
})
const GROUP = 5

// currency-style denominations: banked stars bundle up 5 → 10 → 50 → …, so the
// row stays short however far the prestige cap grows. Bigger bundle, bigger
// sprite; the number label says what it is worth.
const DENOMS = [500, 100, 50, 10, 5, 1]
const SIZE: Record<number, string> = {
  500: 'h-12 w-12',
  100: 'h-11 w-11',
  50: 'h-10 w-10',
  10: 'h-9 w-9',
  5: 'h-8 w-8',
  1: 'h-5 w-5',
}

// visuals cap at prismatic even though prestige keeps counting
const skin = computed(() => Math.min(props.prestige, cfg().prestigeSkinCap))
const filledClass = computed(() =>
  skin.value > 0 ? `star-r${skin.value}` : 'drop-shadow-[0_0_6px_#facc15]',
)

function bundles(n: number): number[] {
  const out: number[] = []
  for (const d of DENOMS) while (n >= d) (out.push(d), (n -= d))
  return out
}

// only the active group of 5 — the one the NEXT star lands in — renders as
// singles, so per-click the row never reflows; everything banked below it
// bundles up, and re-bundling happens only when crossing a denomination line.
// The win state banks everything (no next star) and that screen changes anyway.
const win = computed(() => props.stars >= props.max)
const done = computed(() => (win.value ? props.stars : props.stars - (props.stars % GROUP)))
const activeFilled = computed(() => props.stars - done.value)
const filledBundles = computed(() => bundles(done.value))
// the untouched remainder past the active group, dimmed like empty stars
const restBundles = computed(() =>
  bundles(Math.max(0, props.max - done.value - (win.value ? 0 : GROUP))),
)
</script>

<template>
  <div class="flex max-w-sm flex-wrap items-center justify-center gap-1.5">
    <span v-for="(d, i) in filledBundles" :key="`f${i}-${d}`" class="relative">
      <img src="/star.png" alt="★" :class="[SIZE[d], 'star-pop', filledClass]" />
      <span
        v-if="d > 1"
        class="absolute inset-0 flex items-center justify-center pb-0.5 text-[11px] font-black text-black/80"
      >
        {{ d }}
      </span>
    </span>
    <!-- the group in progress: individual stars -->
    <template v-if="!win">
      <img
        v-for="i in GROUP"
        :key="`a-${i}-${i <= activeFilled}`"
        src="/star.png"
        alt="★"
        class="h-5 w-5 transition-all"
        :class="i <= activeFilled ? ['star-pop', filledClass] : 'opacity-15 grayscale'"
      />
    </template>
    <span v-for="(d, i) in restBundles" :key="`r${i}-${d}`" class="relative">
      <img src="/star.png" alt="★" :class="[SIZE[d], 'opacity-15 grayscale']" />
      <span
        v-if="d > 1"
        class="absolute inset-0 flex items-center justify-center pb-0.5 text-[11px] font-black text-slate-500"
      >
        {{ d }}
      </span>
    </span>
    <!-- bundles hide the raw count, so spell out the distance to prestige -->
    <span
      class="ml-1 font-mono text-xs text-slate-400"
      :title="t('toPrestige').replace('{n}', String(Math.max(0, max - stars)))"
    >
      {{ stars }}/{{ max }}
    </span>
  </div>
</template>
