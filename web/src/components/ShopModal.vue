<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  state,
  buySkill,
  buyLottery,
  buyPack,
  loadCards,
  refillQuota,
  skill,
  type Card,
  type SkillKey,
} from '../useGame'
import { cfg } from '../config'
import { t } from '../i18n'
import { play, vibrate } from '../audio'
import { fmtCoins, useCoinCounter } from '../useCoinCounter'
import LotteryModal from './LotteryModal.vue'
import PackModal from './PackModal.vue'

defineProps<{ kind: 'items' | 'skills' }>()
defineEmits<{ close: [] }>()

const ticket = ref<{ prize: number; coins: number } | null>(null)
const shownCoins = useCoinCounter(() => state.value?.coins ?? 0)

const pack = ref<Card[] | null>(null)

async function onPack() {
  const drawn = await buyPack()
  if (drawn?.length) {
    play('coin-use')
    vibrate([8, 15, 12])
    pack.value = drawn
  }
}

async function onRefill() {
  if (await refillQuota()) {
    play('coin-use')
    vibrate([15, 20, 30])
  }
}

async function onLottery() {
  const bought = await buyLottery()
  if (bought) {
    play('coin-use')
    vibrate([8, 15, 12]) // ticket tearing off the roll
    ticket.value = bought
  }
}

// null first so the v-if remounts the modal with fresh scratch/tear state
async function onLotteryAgain() {
  ticket.value = null
  await onLottery()
}

async function onPackAgain() {
  pack.value = null
  await loadCards()
  await onPack()
}

const rows = computed(() => {
  const s = state.value!
  return [
    {
      id: 'tut-shop-charm',
      key: 'charm' as SkillKey,
      icon: '🍀',
      name: t('charmName'),
      desc: t('charmDesc'),
      levelText: `Lv ${s.charmLevel}/${skill('charm').cap}`,
      price: skill('charm').prices[s.charmLevel] ?? 0,
      capped: s.charmLevel >= skill('charm').cap,
    },
    {
      id: 'tut-shop-stamina',
      key: 'stamina' as SkillKey,
      icon: '🔋',
      name: t('staminaName'),
      desc: t('staminaDesc').replace('{n}', String(cfg().stamina.bonusPct)),
      levelText: `Lv ${s.staminaLevel}/${skill('stamina').cap}`,
      price: skill('stamina').prices[s.staminaLevel] ?? 0,
      capped: s.staminaLevel >= skill('stamina').cap,
    },
    {
      id: 'tut-shop-headstart',
      key: 'headstart' as SkillKey,
      icon: '🚀',
      name: t('headstartName'),
      desc: t('headstartDesc'),
      levelText: `Lv ${s.headstartLevel}/${skill('headstart').cap}`,
      price: skill('headstart').prices[s.headstartLevel] ?? 0,
      capped: s.headstartLevel >= skill('headstart').cap,
    },
    {
      id: 'tut-shop-magnet',
      key: 'magnet' as SkillKey,
      icon: '🧲',
      name: t('magnetName'),
      desc: t('magnetDesc').replace('{n}', String(cfg().magnet.bonusPct)),
      levelText: `Lv ${s.magnetLevel}/${skill('magnet').cap}`,
      price: skill('magnet').prices[s.magnetLevel] ?? 0,
      capped: s.magnetLevel >= skill('magnet').cap,
    },
    {
      id: 'tut-shop-golden',
      key: 'golden' as SkillKey,
      icon: '🪙',
      name: t('goldenName'),
      desc: t('goldenDesc').replace('{n}', String(cfg().golden.bonusPct)),
      levelText: `Lv ${s.goldenLevel}/${skill('golden').cap}`,
      price: skill('golden').prices[s.goldenLevel] ?? 0,
      capped: s.goldenLevel >= skill('golden').cap,
    },
  ]
})

async function onBuy(key: SkillKey) {
  if (await buySkill(key)) {
    play('coin-use')
    vibrate([10, 15, 25])
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    @click.self="$emit('close')"
  >
    <div v-if="state" class="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">
        {{ kind === 'items' ? `🎁 ${t('itemShop')}` : `📈 ${t('skillShop')}` }}
        <span class="ml-2 font-mono text-yellow-300 light:text-yellow-600" :title="String(state?.coins ?? 0)">
          💰 <span class="inline-block min-w-[5ch] text-left">{{ fmtCoins(shownCoins) }}</span>
        </span>
      </h2>

      <template v-if="kind === 'items'">
      <div
        id="tut-shop-lottery"
        class="flex items-center gap-3 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2"
      >
        <span class="text-xl">🎟️</span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-bold text-slate-200">{{ t('lotteryName') }}</p>
          <p class="text-xs text-slate-500">{{ t('lotteryDesc') }}</p>
        </div>
        <button
          class="rounded-lg bg-rose-400 px-3 py-1.5 font-mono text-xs font-bold text-black hover:bg-rose-300 disabled:opacity-40"
          :disabled="state.coins < cfg().lottery.price"
          @click="onLottery"
        >
          {{ cfg().lottery.price }}💰
        </button>
      </div>

      <div
        id="tut-shop-pack"
        class="flex items-center gap-3 rounded-lg border border-violet-500/40 bg-violet-500/10 px-3 py-2"
      >
        <span class="text-xl">🎴</span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-bold text-slate-200">{{ t('packName') }}</p>
          <p class="text-xs text-slate-500">{{ t('packDesc') }}</p>
        </div>
        <button
          class="rounded-lg bg-violet-400 px-3 py-1.5 font-mono text-xs font-bold text-black hover:bg-violet-300 disabled:opacity-40"
          :disabled="state.coins < cfg().pack.price"
          @click="onPack"
        >
          {{ cfg().pack.price }}💰
        </button>
      </div>

      <div
        id="tut-shop-refill"
        class="flex items-center gap-3 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2"
      >
        <span class="text-xl">⏰</span>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-bold text-slate-200">{{ t('refillName') }}</p>
          <p class="text-xs text-slate-500">
            {{ state.refillsLeft > 0 ? t('refillDesc').replace('{n}', String(state.refillsLeft)) : t('refillUsed') }}
          </p>
        </div>
        <button
          class="rounded-lg bg-emerald-400 px-3 py-1.5 font-mono text-xs font-bold text-black hover:bg-emerald-300 disabled:opacity-40"
          :disabled="state.coins < cfg().refillPrice || state.quotaLeft >= state.quota || state.refillsLeft <= 0"
          @click="onRefill"
        >
          {{ cfg().refillPrice }}💰
        </button>
      </div>
      </template>

      <div v-else class="flex flex-col gap-2">
        <div
          v-for="row in rows"
          :key="row.key"
          :id="row.id"
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
            class="rounded-lg bg-yellow-400 px-3 py-1.5 font-mono text-xs font-bold text-black hover:bg-yellow-300 disabled:opacity-40"
            :disabled="row.capped || state.coins < row.price"
            @click="onBuy(row.key)"
          >
            {{ row.capped ? t('maxLevel') : `${row.price}💰` }}
          </button>
        </div>
      </div>

      <button
        data-space
        class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
        @click="$emit('close')"
      >
        OK
      </button>
    </div>

    <LotteryModal
      v-if="ticket"
      :prize="ticket.prize"
      :final-coins="ticket.coins"
      @again="onLotteryAgain"
      @close="ticket = null"
    />
    <PackModal v-if="pack" :cards="pack" @again="onPackAgain" @close="pack = null; loadCards()" />
  </div>
</template>
