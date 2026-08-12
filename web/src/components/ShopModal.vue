<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  state,
  sellStreak,
  buySkill,
  buyLottery,
  buyPack,
  loadCards,
  refillQuota,
  skill,
  streakValue,
  type Card,
  type SkillKey,
} from '../useGame'
import { cfg } from '../config'
import { t } from '../i18n'
import { play, vibrate } from '../audio'
import { burst } from '../particles'
import { COIN_COLORS, useCoinCounter } from '../useCoinCounter'
import LotteryModal from './LotteryModal.vue'
import PackModal from './PackModal.vue'

defineEmits<{ close: [] }>()

const ticket = ref<{ prize: number; coins: number } | null>(null)
const coinEl = ref<HTMLElement | null>(null)
const shownCoins = useCoinCounter(() => state.value?.coins ?? 0)

function coinBurst(count: number) {
  const r = coinEl.value?.getBoundingClientRect()
  if (r) burst(r.left + r.width / 2, r.top + r.height / 2, COIN_COLORS, count)
}

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

const sellValue = computed(() =>
  state.value ? streakValue(state.value.stars, state.value.headstartLevel) : 0,
)

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
      id: 'tut-shop-headstart',
      key: 'headstart' as SkillKey,
      icon: '🚀',
      name: t('headstartName'),
      desc: t('headstartDesc'),
      levelText: `Lv ${s.headstartLevel}/${skill('headstart').cap}`,
      price: skill('headstart').prices[s.headstartLevel] ?? 0,
      capped: s.headstartLevel >= skill('headstart').cap,
    },
  ]
})

async function onSell() {
  if (!sellValue.value) return
  const gained = await sellStreak()
  if (gained) {
    play('streak-sell')
    vibrate([15, 20, 35]) // coins clattering in
    // more coins, bigger shower
    coinBurst(Math.min(30 + Math.floor(gained / 2), 90))
  }
}

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
        🛒 {{ t('shop') }}
        <span ref="coinEl" class="ml-2 font-mono text-yellow-300">💰 {{ shownCoins }}</span>
      </h2>

      <button
        id="tut-shop-sell"
        class="flex items-center justify-between rounded-lg border border-yellow-500/40 bg-yellow-400/10 px-4 py-2 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20 disabled:opacity-40"
        :disabled="!sellValue || state.win"
        @click="onSell"
      >
        <span>⭐ {{ t('sellStreak') }} (★{{ state.stars }})</span>
        <span class="font-mono">+{{ sellValue }}💰</span>
      </button>
      <p class="text-center text-xs text-slate-500">{{ state.win ? t('sellAtWin') : t('sellDesc') }}</p>

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
          class="rounded-lg bg-rose-400 px-3 py-1.5 font-mono text-xs font-bold text-slate-900 hover:bg-rose-300 disabled:opacity-40"
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
          class="rounded-lg bg-violet-400 px-3 py-1.5 font-mono text-xs font-bold text-slate-900 hover:bg-violet-300 disabled:opacity-40"
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
          <p class="text-xs text-slate-500">{{ state.refillUsed ? t('refillUsed') : t('refillDesc') }}</p>
        </div>
        <button
          class="rounded-lg bg-emerald-400 px-3 py-1.5 font-mono text-xs font-bold text-slate-900 hover:bg-emerald-300 disabled:opacity-40"
          :disabled="state.coins < cfg().refillPrice || state.quotaLeft >= state.quota || state.refillUsed"
          @click="onRefill"
        >
          {{ cfg().refillPrice }}💰
        </button>
      </div>

      <div class="flex flex-col gap-2">
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
