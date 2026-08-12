<script setup lang="ts">
import { leaderboard } from '../useGame'
import { cfg } from '../config'
import { t } from '../i18n'
import TierBadge from './TierBadge.vue'

// Prestige runs past the last distinct star skin, so anything above the cap is
// another lap of the top tier. Lap 1 is the skin itself and needs no counter.
const lap = (prestige: number) => prestige - cfg().prestigeSkinCap + 1
const skin = (prestige: number) => `star-r${Math.min(prestige, cfg().prestigeSkinCap)}`
</script>

<template>
  <section class="rounded-xl border border-slate-700/60 bg-slate-900/70 p-4 backdrop-blur">
    <h2 class="mb-3 text-sm font-bold uppercase tracking-widest text-slate-400">
      🏆 {{ t('leaderboard') }}
    </h2>
    <p v-if="leaderboard.length === 0" class="text-sm text-slate-500">{{ t('empty') }}</p>
    <ol v-else class="space-y-1">
      <!-- two lines per entry: the name gets the full column width instead of
           competing with the tier badge and star count for one cramped row -->
      <li
        v-for="(e, i) in leaderboard"
        :key="e.nickname + i"
        class="flex items-center gap-2 rounded-md px-2 py-1.5"
        :class="i === 0 ? 'bg-yellow-400/10' : 'odd:bg-slate-800/40'"
      >
        <span class="w-5 shrink-0 text-right font-mono text-xs text-slate-500">{{ i + 1 }}</span>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-slate-200" :title="e.nickname">
            {{ e.nickname }}
          </p>
          <div class="mt-1 flex items-center gap-1.5">
            <TierBadge :tier="e.tier" small />
            <span v-if="e.prestige > 0" class="inline-flex items-center gap-0.5" :title="t('prestige')">
              <img src="/star.png" alt="" class="h-3.5 w-3.5" :class="skin(e.prestige)" />
              <!-- ×N, not -N: a leading minus read as a negative number -->
              <span
                v-if="lap(e.prestige) > 1"
                class="font-mono text-[10px] font-bold text-fuchsia-300"
              >
                ×{{ lap(e.prestige) }}
              </span>
            </span>
          </div>
        </div>
        <span class="shrink-0 self-center font-mono text-sm font-bold text-yellow-400">
          ★{{ e.stars }}
        </span>
      </li>
    </ol>
    <p class="mt-3 text-center text-[10px] leading-snug text-slate-600">{{ t('rankOrder') }}</p>
  </section>
</template>
