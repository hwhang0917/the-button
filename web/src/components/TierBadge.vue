<script setup lang="ts">
import { computed } from 'vue'
import { TIER_COLORS, type Tier } from '../tiers'
import { theme } from '../theme'
import { t } from '../i18n'

const props = defineProps<{ tier: Tier; small?: boolean }>()

// the near-white tier colors (silver especially) vanish on the light
// background — darker stand-ins keep the badge legible there. Card faces
// keep TIER_COLORS untouched: their ground is always dark.
const LIGHT_TIER_COLORS: Partial<Record<Tier, string>> = {
  unrank: '#6b7280',
  silver: '#64748b',
  gold: '#b45309',
  platinum: '#0e7490',
}

const color = computed(
  () => (theme.value === 'light' && LIGHT_TIER_COLORS[props.tier]) || TIER_COLORS[props.tier],
)
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-full font-bold uppercase tracking-wider"
    :class="small ? 'px-2 py-0.5 text-[10px]' : 'px-4 py-1 text-sm'"
    :style="{
      color,
      border: `1px solid ${color}66`,
      background: `${color}1a`,
      textShadow: `0 0 12px ${color}88`,
    }"
  >
    {{ t('tier')[tier] }}
  </span>
</template>
