<script setup lang="ts">
import { computed } from 'vue'
import { armTalisman, cards, state } from '../useGame'
import { CARD_EMOJI, RARITIES, TIER_COLORS, type Rarity, type Tier } from '../tiers'
import { cardName, effectText } from '../cards'
import { t } from '../i18n'
import { play, vibrate } from '../audio'

const emit = defineEmits<{ close: [] }>()

// every card in hand is usable now — the tier gate is gone. Strongest band
// first so the good stuff isn't buried under a pile of commons.
const usable = computed(() =>
  cards.value
    .filter((c) => c.count > 0)
    .sort((a, b) => RARITIES.indexOf(b.rarity) - RARITIES.indexOf(a.rarity)),
)

async function onPick(tier: Tier, rarity: Rarity) {
  if (await armTalisman(tier, rarity)) {
    play('talisman')
    vibrate([10, 20, 25])
    emit('close')
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    @click.self="$emit('close')"
  >
    <div v-if="state" class="flex max-h-[80vh] w-full max-w-sm flex-col gap-3 rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">🃏 {{ t('talismanPick') }}</h2>
      <p class="text-center text-xs text-slate-500">{{ t('talismanNextClick') }}</p>

      <p v-if="usable.length === 0" class="py-4 text-center text-sm text-slate-500">
        {{ t('talismanNone') }}
      </p>

      <div class="flex min-h-0 flex-col gap-2 overflow-y-auto">
        <button
          v-for="c in usable"
          :key="`${c.tier}/${c.rarity}`"
          class="flex items-center gap-3 rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2 text-left hover:border-amber-400/60 hover:bg-slate-800"
          @click="onPick(c.tier, c.rarity)"
        >
          <span class="text-2xl leading-none">{{ CARD_EMOJI[c.tier][c.rarity] }}</span>
          <span class="min-w-0 flex-1">
            <span class="block text-sm font-bold text-slate-200">
              {{ cardName(c.tier, c.rarity) }}
              <span class="ml-1 text-[10px] font-bold" :style="{ color: TIER_COLORS[c.tier] }">
                {{ t('rarity')[c.rarity] }}
              </span>
              <span class="ml-1 font-mono text-xs text-slate-400">×{{ c.count }}</span>
            </span>
            <span class="block text-xs text-slate-500">{{ effectText(c.tier, c.rarity) }}</span>
          </span>
        </button>
      </div>

      <button
        class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        {{ t('later') }}
      </button>
    </div>
  </div>
</template>
