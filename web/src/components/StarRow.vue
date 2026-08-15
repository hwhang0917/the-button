<script setup lang="ts">
import { computed } from 'vue'
import { cfg } from '../config'
import { t } from '../i18n'

const props = withDefaults(defineProps<{ stars: number; prestige?: number; max?: number }>(), {
  prestige: 0,
  max: 15,
})
const GROUP = 5

// visuals cap at prismatic even though prestige keeps counting
const skin = computed(() => Math.min(props.prestige, cfg().prestigeSkinCap))
const filledClass = computed(() =>
  skin.value > 0 ? `star-r${skin.value}` : 'drop-shadow-[0_0_6px_#facc15]',
)

// groups of 5: complete and untouched groups collapse into one big "5"-star so
// the row never wraps on narrow screens; only the active group shows singles
const groups = computed(() =>
  Array.from({ length: Math.ceil(props.max / GROUP) }, (_, i) =>
    Math.max(0, Math.min(GROUP, props.stars - i * GROUP)),
  ),
)

// exactly one group renders as singles — the one the NEXT star lands in, even
// when it is freshly empty. Crossing a multiple of 5 then keeps the row the
// same width, so the wrapping status row never reflows mid-click-streak. Only
// the win state (all groups big) narrows, and that screen changes anyway.
const active = computed(() =>
  props.stars >= props.max ? -1 : Math.floor(props.stars / GROUP),
)
</script>

<template>
  <div class="flex max-w-xs items-center justify-center gap-1.5">
    <template v-for="(filled, gi) in groups" :key="`${gi}-${filled}`">
      <!-- complete or untouched group: one large star worth 5 -->
      <span v-if="gi !== active" class="relative">
        <img
          src="/star.png"
          alt="★5"
          class="h-8 w-8"
          :class="filled === GROUP ? ['star-pop', filledClass] : 'opacity-15 grayscale'"
        />
        <span
          class="absolute inset-0 flex items-center justify-center pb-0.5 text-[11px] font-black"
          :class="filled === GROUP ? 'text-black/80' : 'text-slate-500'"
        >
          5
        </span>
      </span>
      <!-- the group in progress: individual stars -->
      <template v-else>
        <img
          v-for="i in GROUP"
          :key="`${gi}-${i}-${i <= filled}`"
          src="/star.png"
          alt="★"
          class="h-5 w-5 transition-all"
          :class="i <= filled ? ['star-pop', filledClass] : 'opacity-15 grayscale'"
        />
      </template>
    </template>
    <!-- the groups hide the raw count, so spell out the distance to prestige -->
    <span
      class="ml-1 font-mono text-xs text-slate-400"
      :title="t('toPrestige').replace('{n}', String(Math.max(0, max - stars)))"
    >
      {{ stars }}/{{ max }}
    </span>
  </div>
</template>
