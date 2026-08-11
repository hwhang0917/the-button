<script setup lang="ts">
import { computed } from 'vue'
import { armTalisman, cards, state } from '../useGame'
import { TIER_COLORS, type Rarity } from '../tiers'
import { t } from '../i18n'
import { play, vibrate } from '../audio'

const emit = defineEmits<{ close: [] }>()

const RARITY_FX: Record<Rarity, string> = {
  common: '',
  rare: 'star-r1',
  holo: 'star-r2',
  prismatic: 'star-r3',
}

// only cards usable right now: current tier, at least one copy
const usable = computed(() =>
  cards.value.filter((c) => c.tier === state.value?.tier && c.count > 0),
)

async function onPick(rarity: Rarity) {
  const s = state.value
  if (!s) return
  if (await armTalisman(s.tier, rarity)) {
    play('switch')
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
    <div v-if="state" class="flex w-full max-w-sm flex-col gap-3 rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">
        🃏 {{ t('talismanPick') }}
        <span class="ml-1 text-sm" :style="{ color: TIER_COLORS[state.tier] }">{{ t('tier')[state.tier] }}</span>
      </h2>

      <p v-if="usable.length === 0" class="py-4 text-center text-sm text-slate-500">
        {{ t('talismanNone') }}
      </p>

      <button
        v-for="c in usable"
        :key="c.rarity"
        class="flex items-center gap-3 rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2 text-left hover:border-amber-400/60 hover:bg-slate-800"
        @click="onPick(c.rarity as Rarity)"
      >
        <img src="/star.png" class="h-6 w-6" :class="RARITY_FX[c.rarity as Rarity]" alt="" />
        <span class="min-w-0 flex-1">
          <span class="block text-sm font-bold text-slate-200">
            {{ t('rarity')[c.rarity as Rarity] }}
            <span class="ml-1 font-mono text-xs text-slate-400">×{{ c.count }}</span>
          </span>
          <span class="block text-xs text-slate-500">{{ t('talEffect')[c.rarity as Rarity] }}</span>
        </span>
      </button>

      <button
        class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        {{ t('later') }}
      </button>
    </div>
  </div>
</template>
