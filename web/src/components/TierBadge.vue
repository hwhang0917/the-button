<script setup lang="ts">
import { TIER_COLORS, type Tier } from '../tiers'
import { t } from '../i18n'

defineProps<{ tier: Tier; small?: boolean }>()
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-full font-bold uppercase tracking-wider"
    :class="small ? 'px-2 py-0.5 text-[10px]' : 'px-4 py-1 text-sm'"
    :style="{
      color: TIER_COLORS[tier],
      border: `1px solid ${TIER_COLORS[tier]}66`,
      background: `${TIER_COLORS[tier]}1a`,
      textShadow: `0 0 12px ${TIER_COLORS[tier]}88`,
    }"
  >
    {{ t('tier')[tier] }}
  </span>
</template>
