<script setup lang="ts">
import 'katex/dist/katex.min.css'
import { cfg } from '../config'
import { t } from '../i18n'

defineEmits<{ close: [] }>()

// the live table, so retuning chance_table in config.yml updates this popup too
const CHANCE_TABLE = cfg().chanceTable

// equations pre-rendered by KaTeX at build time (vite.config.ts); mirrors
// game/resolve.go: EffChanceFor + card bonus, roll, GainFor, fail chain
const { chance: eqChance, roll: eqRoll, gain: eqGain, next: eqNext, jackpot: eqJackpot, fail: eqFail } = __ODDS_MATH__
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    @click.self="$emit('close')"
  >
    <div class="flex max-h-[85vh] w-full max-w-md flex-col gap-3 overflow-y-auto rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">🎲 {{ t('oddsTitle') }}</h2>

      <ul class="flex flex-col gap-1.5 text-sm text-slate-300">
        <li v-for="(line, i) in t('oddsSimple')" :key="i">{{ line }}</li>
      </ul>

      <div class="overflow-x-auto rounded-lg bg-slate-800/60 p-2">
        <table class="mx-auto text-center font-mono text-[10px] text-slate-300">
          <tr>
            <td class="pr-2 text-slate-500">★s</td>
            <td v-for="(_, i) in CHANCE_TABLE" :key="i" class="px-1 text-slate-500">{{ i }}</td>
          </tr>
          <tr>
            <td class="pr-2 text-slate-500">base</td>
            <td v-for="(v, i) in CHANCE_TABLE" :key="i" class="px-1 text-yellow-300 light:text-yellow-700">{{ v }}</td>
          </tr>
        </table>
      </div>

      <details class="group rounded-lg border border-slate-700/60 bg-slate-800/40 p-3">
        <summary
          class="cursor-pointer list-none text-sm font-bold text-slate-300 select-none [&::-webkit-details-marker]:hidden"
        >
          {{ t('oddsMore') }} <span class="float-right transition-transform group-open:rotate-180">▾</span>
        </summary>
        <div class="mt-3 flex flex-col gap-3">
          <p class="text-xs text-slate-400">{{ t('oddsVars') }}</p>

          <div class="odds-math" v-html="eqChance"></div>
          <p class="text-xs text-slate-500">{{ t('oddsChanceDesc') }}</p>

          <div class="odds-math" v-html="eqRoll"></div>
          <p class="text-xs text-slate-500">{{ t('oddsRollDesc') }}</p>

          <div class="odds-math" v-html="eqGain"></div>
          <div class="odds-math" v-html="eqNext"></div>
          <div class="odds-math" v-html="eqJackpot"></div>
          <p class="text-xs text-slate-500">{{ t('oddsGainDesc') }}</p>

          <div class="odds-math" v-html="eqFail"></div>
          <p class="text-xs text-slate-500">{{ t('oddsFailDesc') }}</p>
        </div>
      </details>

      <button
        class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        OK
      </button>
    </div>
  </div>
</template>

<style scoped>
.odds-math {
  overflow-x: auto;
  /* the slate ladder flips with the theme; a raw hex would vanish in light mode */
  color: var(--color-slate-200);
}
.odds-math :deep(.katex-display) {
  margin: 0.25rem 0;
}
</style>
