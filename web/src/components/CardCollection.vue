<script setup lang="ts">
import { computed } from 'vue'
import { cards, type Card } from '../useGame'
import { RARITIES, TIERS, TIER_COLORS } from '../tiers'
import { t } from '../i18n'

defineEmits<{ view: [Card] }>()

const owned = computed(() => {
  const m = new Map<string, number>()
  for (const c of cards.value) m.set(`${c.tier}/${c.rarity}`, c.count)
  return m
})
</script>

<template>
  <section class="rounded-xl border border-slate-700/60 bg-slate-900/70 p-4 backdrop-blur">
    <h2 class="mb-3 text-sm font-bold uppercase tracking-widest text-slate-400">
      🃏 {{ t('collection') }} ({{ cards.length }}/24)
    </h2>
    <div class="grid grid-cols-4 gap-2">
      <template v-for="tier in TIERS" :key="tier">
        <div
          v-for="rarity in RARITIES"
          :key="rarity"
          class="relative flex aspect-[3/4] flex-col items-center justify-center rounded-md border text-center"
          :class="owned.has(`${tier}/${rarity}`) ? 'cursor-pointer' : 'border-slate-800 bg-slate-950/60'"
          @click="owned.has(`${tier}/${rarity}`) && $emit('view', { tier, rarity })"
          :style="
            owned.has(`${tier}/${rarity}`)
              ? {
                  borderColor: TIER_COLORS[tier],
                  background: `linear-gradient(160deg, ${TIER_COLORS[tier]}33, #0f172a)`,
                }
              : {}
          "
        >
          <template v-if="owned.has(`${tier}/${rarity}`)">
            <img src="/star.png" class="h-5 w-5" alt="" />
            <span class="mt-1 text-[9px] font-bold leading-tight" :style="{ color: TIER_COLORS[tier] }">
              {{ t('tier')[tier] }}
            </span>
            <span class="text-[8px] uppercase text-slate-400">{{ t('rarity')[rarity] }}</span>
            <span
              v-if="(owned.get(`${tier}/${rarity}`) ?? 0) > 1"
              class="absolute right-0.5 top-0.5 rounded bg-slate-900/90 px-1 text-[8px] font-mono text-slate-300"
            >
              ×{{ owned.get(`${tier}/${rarity}`) }}
            </span>
          </template>
          <span v-else class="text-lg text-slate-700">?</span>
        </div>
      </template>
    </div>
  </section>
</template>
