<script setup lang="ts">
import { computed } from 'vue'
import type { Card } from '../useGame'
import { CARD_EMOJI, TIER_COLORS } from '../tiers'
import { cardName, effectText } from '../cards'
import { t } from '../i18n'

const props = defineProps<{ card: Card }>()

const name = computed(() => cardName(props.card.tier, props.card.rarity))
const effect = computed(() => effectText(props.card.tier, props.card.rarity))

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
  // black over a near-black card — they need luminance underneath to ignite.
  // the trailing solid layer keeps the card fully opaque
  background: `linear-gradient(165deg, ${TIER_COLORS[props.card.tier]}88, #475569 60%, ${TIER_COLORS[props.card.tier]}44), #334155`,
  boxShadow: `0 0 50px ${TIER_COLORS[props.card.tier]}66`,
}))
</script>

<template>
  <div
    class="card relative flex h-80 w-56 select-none flex-col items-center justify-between rounded-2xl border-2 p-5"
    :data-rarity="DATA_RARITY[card.rarity]"
    :style="faceStyle"
  >
    <span class="self-end rounded-full bg-black/40 px-2 py-0.5 text-[10px] font-bold uppercase tracking-widest text-zinc-200">
      {{ t('rarity')[card.rarity] }}
    </span>
    <!-- dark disc keeps the emoji legible: the color-dodge foil stays
         dark over dark pixels, so the art pops even on bright washes -->
    <span
      class="flex h-24 w-24 items-center justify-center rounded-full bg-black/40 text-6xl drop-shadow-[0_2px_8px_rgba(0,0,0,0.8)]"
    >
      {{ CARD_EMOJI[card.tier][card.rarity] }}
    </span>
    <p class="rounded-lg bg-black/30 px-2 py-1 text-center text-[11px] leading-snug text-zinc-100">
      🃏 {{ effect }}
    </p>
    <div class="text-center">
      <p class="text-sm font-black leading-tight text-zinc-50">{{ name }}</p>
      <p class="text-[11px] font-bold uppercase tracking-widest" :style="{ color: TIER_COLORS[card.tier] }">
        {{ t('cardTier')[card.tier] }}
      </p>
    </div>
    <div class="card__shine"></div>
    <div class="card__glare"></div>
  </div>
</template>
