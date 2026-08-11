<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  cards,
  click,
  deletePlayer,
  loadCards,
  loadLeaderboard,
  loadState,
  MAX_RISK,
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

const risk = ref(0)
const busy = ref(false)
const shaking = ref(false)
const flashing = ref(false)
const message = ref('')
const messageColor = ref('text-slate-300')
const droppedCard = ref<Card | null>(null)
const viewedCard = ref<Card | null>(null)
const showNickname = ref(false)
const nicknameDismissed = ref(false)
const menuOpen = ref(false)

async function onDelete() {
  menuOpen.value = false
  if (!confirm(t('deleteConfirm'))) return
  if (await deletePlayer()) location.reload()
}

const now = ref(Date.now())

// ponytail: quota refills on the server's clock hour; client top-of-hour matches
// for whole-hour timezones — pass the server's bucket deadline in /api/state if that breaks
const refillIn = computed(() => {
  const d = new Date(now.value)
  const s = 3599 - d.getMinutes() * 60 - d.getSeconds()
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
})

const disabled = computed(
  () => busy.value || !state.value || state.value.quotaLeft <= 0 || state.value.win,
)
const displayChance = computed(() =>
  state.value ? Math.floor(state.value.chance / (risk.value + 1)) : 0,
)

function setRisk(lvl: number) {
  risk.value = lvl
  play('switch')
}

async function onPress(center: { x: number; y: number }) {
  if (busy.value) return
  busy.value = true
  const result = await click(risk.value)
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
  let lastHour = new Date().getHours()
  setInterval(() => {
    now.value = Date.now()
    const h = new Date().getHours()
    if (h !== lastHour) {
      lastHour = h
      if (state.value && state.value.quotaLeft <= 0) loadState()
    }
  }, 1000)
})
</script>

<template>
  <div :class="{ shake: shaking }" class="min-h-screen text-slate-200">
    <div v-if="flashing" class="flash-red pointer-events-none fixed inset-0 z-30 bg-rose-600"></div>

    <header class="flex items-center justify-between px-4 py-3 sm:px-8">
      <h1 class="text-xl font-black tracking-[0.2em] text-white sm:text-2xl">THE BUTTON</h1>
      <div class="flex items-center gap-3">
        <div v-if="state?.nickname" class="relative">
          <button
            class="max-w-32 truncate text-sm text-slate-400 hover:text-slate-200"
            @click="menuOpen = !menuOpen"
          >
            {{ state.nickname }} ▾
          </button>
          <template v-if="menuOpen">
            <div class="fixed inset-0 z-40" @click="menuOpen = false"></div>
            <div
              class="absolute right-0 top-full z-50 mt-1 w-40 overflow-hidden rounded-lg border border-slate-700 bg-slate-900 text-sm shadow-xl"
            >
              <button
                class="block w-full px-3 py-2 text-left text-slate-300 hover:bg-slate-800"
                @click="menuOpen = false; showNickname = true"
              >
                ✏️ {{ t('changeName') }}
              </button>
              <button
                class="block w-full px-3 py-2 text-left text-rose-400 hover:bg-slate-800"
                @click="onDelete"
              >
                🗑️ {{ t('deleteData') }}
              </button>
            </div>
          </template>
        </div>
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
            :risky="risk > 0"
            :disabled="disabled"
            @press="onPress"
          />

          <p class="h-6 text-center font-bold" :class="messageColor">{{ message }}</p>

          <div class="flex items-center gap-3 select-none">
            <span class="text-sm font-bold" :class="risk ? 'text-rose-400' : 'text-slate-400'">
              🔥 {{ t('riskIt') }}
            </span>
            <div class="flex overflow-hidden rounded-full border border-slate-700">
              <button
                v-for="lvl in MAX_RISK + 1"
                :key="lvl - 1"
                class="px-3 py-1 text-xs font-bold transition-colors"
                :class="
                  risk === lvl - 1
                    ? lvl === 1 ? 'bg-slate-600 text-white' : 'bg-rose-600 text-white'
                    : 'text-slate-400 hover:bg-slate-800'
                "
                @click="setRisk(lvl - 1)"
              >
                {{ lvl === 1 ? 'OFF' : '🔥'.repeat(lvl - 1) }}
              </button>
            </div>
            <span class="h-4 text-xs text-slate-500">
              <template v-if="risk">{{ t('chance') }} 1/{{ risk + 1 }} · ★+{{ risk + 1 }}</template>
            </span>
          </div>

          <p class="text-sm text-slate-400">
            <template v-if="state.quotaLeft > 0">
              {{ t('clicksLeft') }}:
              <span class="font-mono font-bold text-slate-200">{{ state.quotaLeft }}</span>
              / {{ state.quota }}
            </template>
            <template v-else>
              {{ t('quotaExhausted') }}
              <span class="font-mono font-bold text-slate-200">⏳ {{ refillIn }}</span>
            </template>
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
