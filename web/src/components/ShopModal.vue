<script setup lang="ts">
import { computed } from 'vue'
import { state, sellStreak, buySkill, streakValue, SKILLS, type SkillKey } from '../useGame'
import { t } from '../i18n'
import { play } from '../audio'

defineEmits<{ close: [] }>()

const sellValue = computed(() =>
  state.value ? streakValue(state.value.stars, state.value.headstartLevel) : 0,
)

const rows = computed(() => {
  const s = state.value!
  return [
    {
      key: 'shield' as SkillKey,
      icon: '🛡️',
      name: t('shieldName'),
      desc: t('shieldDesc'),
      levelText: `×${s.shieldCharges}`,
      price: SKILLS.shield.prices[0],
      capped: false,
    },
    {
      key: 'charm' as SkillKey,
      icon: '🍀',
      name: t('charmName'),
      desc: t('charmDesc'),
      levelText: `Lv ${s.charmLevel}/${SKILLS.charm.cap}`,
      price: SKILLS.charm.prices[s.charmLevel] ?? 0,
      capped: s.charmLevel >= SKILLS.charm.cap,
    },
    {
      key: 'headstart' as SkillKey,
      icon: '🚀',
      name: t('headstartName'),
      desc: t('headstartDesc'),
      levelText: `Lv ${s.headstartLevel}/${SKILLS.headstart.cap}`,
      price: SKILLS.headstart.prices[s.headstartLevel] ?? 0,
      capped: s.headstartLevel >= SKILLS.headstart.cap,
    },
  ]
})

async function onSell() {
  if (!sellValue.value) return
  if (await sellStreak()) play('switch')
}

async function onBuy(key: SkillKey) {
  if (await buySkill(key)) play('switch')
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    @click.self="$emit('close')"
  >
    <div v-if="state" class="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">
        🛒 {{ t('shop') }}
        <span class="ml-2 font-mono text-yellow-300">💰 {{ state.coins }}</span>
      </h2>

      <button
        class="flex items-center justify-between rounded-lg border border-yellow-500/40 bg-yellow-400/10 px-4 py-2 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20 disabled:opacity-40"
        :disabled="!sellValue"
        @click="onSell"
      >
        <span>⭐ {{ t('sellStreak') }} (★{{ state.stars }})</span>
        <span class="font-mono">+{{ sellValue }}💰</span>
      </button>
      <p class="text-center text-xs text-slate-500">{{ t('sellDesc') }}</p>

      <div class="flex flex-col gap-2">
        <div
          v-for="row in rows"
          :key="row.key"
          class="flex items-center gap-3 rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2"
        >
          <span class="text-xl">{{ row.icon }}</span>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-bold text-slate-200">
              {{ row.name }}
              <span class="ml-1 font-mono text-xs text-slate-400">{{ row.levelText }}</span>
            </p>
            <p class="text-xs text-slate-500">{{ row.desc }}</p>
          </div>
          <button
            class="rounded-lg bg-yellow-400 px-3 py-1.5 font-mono text-xs font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-40"
            :disabled="row.capped || state.coins < row.price"
            @click="onBuy(row.key)"
          >
            {{ row.capped ? t('maxLevel') : `${row.price}💰` }}
          </button>
        </div>
      </div>

      <button
        class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        OK
      </button>
    </div>
  </div>
</template>
