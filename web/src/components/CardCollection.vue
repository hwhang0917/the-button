<script setup lang="ts">
import { computed } from 'vue'
import { cards, type Card } from '../useGame'
import { RARITIES, TIERS, TIER_COLORS, type Rarity } from '../tiers'
import { t } from '../i18n'

// same visual language as the prestige star skins
const RARITY_FX: Record<Rarity, string> = {
  common: '',
  rare: 'star-r1',
  holo: 'star-r2',
  prismatic: 'star-r3',
}

defineEmits<{ view: [Card] }>()

// the parent re-mounts this component when cards change, so the fold state
// has to live outside the component
const FOLD_KEY = 'bt_collection_folded'
const initiallyOpen = !localStorage.getItem(FOLD_KEY)

function onToggle(e: Event) {
  const d = e.target as HTMLDetailsElement
  if (d.open) localStorage.removeItem(FOLD_KEY)
  else localStorage.setItem(FOLD_KEY, '1')
}

const owned = computed(() => {
  const m = new Map<string, number>()
  for (const c of cards.value) m.set(`${c.tier}/${c.rarity}`, c.count)
  return m
})
</script>

<template>
  <details
    :open="initiallyOpen"
    class="group rounded-xl border border-slate-700/60 bg-slate-900/70 p-4 backdrop-blur"
    @toggle="onToggle"
  >
    <summary
      class="flex cursor-pointer list-none items-center justify-between text-sm font-bold uppercase tracking-widest text-slate-400 select-none [&::-webkit-details-marker]:hidden"
    >
      <!-- rows persist at count 0: discovery survives consuming the last copy -->
      <span>🃏 {{ t('collection') }} ({{ cards.length }}/24)</span>
      <span class="transition-transform group-open:rotate-180">▾</span>
    </summary>
    <div class="mt-3 grid grid-cols-4 gap-2">
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
            <div
              class="flex flex-col items-center"
              :class="owned.get(`${tier}/${rarity}`) === 0 ? 'opacity-40 saturate-50' : ''"
            >
              <img src="/star.png" class="h-5 w-5" :class="RARITY_FX[rarity]" alt="" />
              <span class="mt-1 text-[9px] font-bold leading-tight" :style="{ color: TIER_COLORS[tier] }">
                {{ t('tier')[tier] }}
              </span>
              <span class="text-[8px] uppercase text-slate-400">{{ t('rarity')[rarity] }}</span>
            </div>
            <span
              v-if="(owned.get(`${tier}/${rarity}`) ?? 1) !== 1"
              class="absolute right-0.5 top-0.5 rounded bg-slate-900/90 px-1 text-[8px] font-mono"
              :class="owned.get(`${tier}/${rarity}`) === 0 ? 'text-rose-400' : 'text-slate-300'"
            >
              ×{{ owned.get(`${tier}/${rarity}`) }}
            </span>
          </template>
          <span v-else class="text-lg text-slate-700">?</span>
        </div>
      </template>
    </div>
  </details>
</template>
