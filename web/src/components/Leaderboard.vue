<script setup lang="ts">
import { leaderboard } from '../useGame'
import { t } from '../i18n'
import TierBadge from './TierBadge.vue'
</script>

<template>
  <section class="rounded-xl border border-slate-700/60 bg-slate-900/70 p-4 backdrop-blur">
    <h2 class="mb-3 text-sm font-bold uppercase tracking-widest text-slate-400">
      🏆 {{ t('leaderboard') }}
    </h2>
    <p v-if="leaderboard.length === 0" class="text-sm text-slate-500">{{ t('empty') }}</p>
    <ol v-else class="space-y-1.5">
      <li
        v-for="(e, i) in leaderboard"
        :key="e.nickname + i"
        class="flex items-center gap-2 rounded-md px-2 py-1 text-sm"
        :class="i === 0 ? 'bg-yellow-400/10' : 'odd:bg-slate-800/40'"
        :title="`💰${e.earned}`"
      >
        <span class="w-6 text-right font-mono text-slate-500">{{ i + 1 }}</span>
        <span class="flex-1 truncate font-medium text-slate-200">{{ e.nickname }}</span>
        <img
          v-if="e.prestige > 0"
          src="/star.png"
          alt=""
          class="h-4 w-4"
          :class="`star-r${e.prestige}`"
        />
        <TierBadge :tier="e.tier" small />
        <span
          v-if="e.prestige >= 3 && e.stars >= 15"
          class="text-right font-mono text-xs text-yellow-500/80"
        >
          💰{{ e.earned }}
        </span>
        <span v-else class="w-10 text-right font-mono text-yellow-400">★{{ e.stars }}</span>
      </li>
    </ol>
    <p class="mt-3 text-center text-[10px] text-slate-600">{{ t('rankOrder') }}</p>
  </section>
</template>
