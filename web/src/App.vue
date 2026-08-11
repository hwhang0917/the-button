<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  cards,
  click,
  loadCards,
  loadLeaderboard,
  loadState,
  state,
  wouldRank,
  type Card,
} from './useGame'
import { t, lang, toggleLang } from './i18n'
import { play } from './audio'
import { burst, confetti } from './particles'
import { TIER_COLORS } from './tiers'
import TheButton from './components/TheButton.vue'
import StarRow from './components/StarRow.vue'
import TierBadge from './components/TierBadge.vue'
import Leaderboard from './components/Leaderboard.vue'
import CardCollection from './components/CardCollection.vue'
import CardReveal from './components/CardReveal.vue'
import NicknameModal from './components/NicknameModal.vue'

const risky = ref(false)
const busy = ref(false)
const shaking = ref(false)
const flashing = ref(false)
const message = ref('')
const messageColor = ref('text-slate-300')
const droppedCard = ref<Card | null>(null)
const viewedCard = ref<Card | null>(null)
const showNickname = ref(false)
const nicknameDismissed = ref(false)

const disabled = computed(
  () => busy.value || !state.value || state.value.quotaLeft <= 0 || state.value.win,
)
const displayChance = computed(() => {
  if (!state.value) return 0
  return risky.value ? Math.floor(state.value.chance / 2) : state.value.chance
})

function toggleRisky() {
  risky.value = !risky.value
  play('switch')
}

async function onPress(center: { x: number; y: number }) {
  if (busy.value) return
  busy.value = true
  const result = await click(risky.value)
  busy.value = false
  if (!result) return

  if (result.success) {
    message.value = result.win ? t('win') : result.tierUp ? t('tierUp') : t('success')
    messageColor.value = result.win ? 'text-yellow-300' : 'text-emerald-400'
    burst(center.x, center.y, [TIER_COLORS[result.tier], '#ffffff', '#facc15'], result.tierUp ? 120 : 60)
    play(result.win ? 'win' : `success_${result.tier}`)
    if (result.win) confetti()
    if (!state.value?.nickname && !nicknameDismissed.value && wouldRank(result.stars)) {
      showNickname.value = true
    }
  } else {
    message.value = t('fail')
    messageColor.value = 'text-rose-400'
    play('fail')
    shaking.value = true
    flashing.value = true
    setTimeout(() => (shaking.value = false), 500)
    setTimeout(() => (flashing.value = false), 550)
  }

  loadLeaderboard()
  if (result.card) {
    await loadCards()
    // let the success burst land before the reveal takes over
    setTimeout(() => (droppedCard.value = result.card), 500)
  }
}

onMounted(() => {
  loadState()
  loadLeaderboard()
  loadCards()
})
</script>

<template>
  <div :class="{ shake: shaking }" class="min-h-screen text-slate-200">
    <div v-if="flashing" class="flash-red pointer-events-none fixed inset-0 z-30 bg-rose-600"></div>

    <header class="flex items-center justify-between px-4 py-3 sm:px-8">
      <h1 class="text-xl font-black tracking-[0.2em] text-white sm:text-2xl">THE BUTTON</h1>
      <div class="flex items-center gap-3">
        <button
          v-if="state?.nickname"
          class="max-w-32 truncate text-sm text-slate-400 hover:text-slate-200"
          @click="showNickname = true"
        >
          {{ state.nickname }}
        </button>
        <button
          class="rounded-full border border-slate-600 px-3 py-1 text-xs font-bold text-slate-300 hover:bg-slate-800"
          @click="toggleLang(); play('switch')"
        >
          {{ lang === 'ko' ? 'EN' : '한국어' }}
        </button>
      </div>
    </header>

    <main class="mx-auto grid max-w-6xl gap-6 px-4 pb-12 lg:grid-cols-[280px_1fr_280px]">
      <Leaderboard class="order-2 lg:order-1" />

      <div class="order-1 flex flex-col items-center gap-5 pt-4 lg:order-2">
        <p class="text-sm text-slate-400">{{ t('subtitle') }}</p>

        <template v-if="state">
          <TierBadge :tier="state.tier" />
          <StarRow :stars="state.stars" />

          <TheButton
            :tier="state.tier"
            :chance="displayChance"
            :risky="risky"
            :disabled="disabled"
            @press="onPress"
          />

          <p class="h-6 text-center font-bold" :class="messageColor">{{ message }}</p>

          <label class="flex cursor-pointer items-center gap-2 select-none">
            <input type="checkbox" :checked="risky" class="peer sr-only" @change="toggleRisky" />
            <span
              class="flex h-6 w-11 items-center rounded-full bg-slate-700 px-0.5 transition-colors peer-checked:bg-rose-600"
            >
              <span
                class="h-5 w-5 rounded-full bg-white transition-transform"
                :class="risky ? 'translate-x-5' : ''"
              ></span>
            </span>
            <span class="text-sm font-bold" :class="risky ? 'text-rose-400' : 'text-slate-400'">
              🔥 {{ t('riskIt') }}
            </span>
            <span class="text-xs text-slate-500">{{ t('riskDesc') }}</span>
          </label>

          <p class="text-sm text-slate-400">
            <template v-if="state.quotaLeft > 0">
              {{ t('clicksLeft') }}:
              <span class="font-mono font-bold text-slate-200">{{ state.quotaLeft }}</span>
              / {{ state.quota }}
            </template>
            <template v-else>{{ t('quotaExhausted') }}</template>
          </p>

          <p class="text-xs text-slate-500">{{ t('best') }}: ★{{ state.bestStars }}</p>
        </template>
      </div>

      <CardCollection class="order-3" :key="cards.length" @view="viewedCard = $event" />
    </main>

    <img
      v-if="state && state.stars >= 13"
      src="/stich.gif"
      alt="pet"
      class="pet-bounce fixed bottom-4 right-4 z-20 h-24 w-24 object-contain drop-shadow-[0_0_15px_#a78bfa]"
    />

    <CardReveal v-if="droppedCard" :card="droppedCard" @close="droppedCard = null" />
    <CardReveal v-if="viewedCard" :card="viewedCard" :drop="false" @close="viewedCard = null" />
    <NicknameModal
      v-if="showNickname"
      @close="showNickname = false; nicknameDismissed = true"
    />
  </div>
</template>
