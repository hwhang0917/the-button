<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  cards,
  click,
  deletePlayer,
  effChance,
  gainFor,
  loadCards,
  loadLeaderboard,
  loadState,
  MAX_RISK,
  state,
  type Card,
} from './useGame'
import { t, lang, toggleLang } from './i18n'
import { play, preloadAudio, soundCount } from './audio'
import { burst, confetti } from './particles'
import { TIER_COLORS } from './tiers'
import TheButton from './components/TheButton.vue'
import StarRow from './components/StarRow.vue'
import TierBadge from './components/TierBadge.vue'
import Leaderboard from './components/Leaderboard.vue'
import CardCollection from './components/CardCollection.vue'
import CardReveal from './components/CardReveal.vue'
import NicknameModal from './components/NicknameModal.vue'
import LinkModal from './components/LinkModal.vue'
import ShopModal from './components/ShopModal.vue'

const IMAGE_ASSETS = ['/wallpaper.jpg', '/star.png', '/stich.gif']
const AUDIO_PRELOAD_TIMEOUT_MS = 4000
// images + sounds + the three initial API calls
const loadTotal = IMAGE_ASSETS.length + soundCount + 3

const ready = ref(false)
const loadProgress = ref(0)

function preloadImage(src: string): Promise<void> {
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = img.onerror = () => resolve()
    img.src = src
  })
}

const risk = ref(0)
const busy = ref(false)
const shaking = ref(false)
const flashing = ref(false)
const message = ref('')
const messageColor = ref('text-slate-300')
const droppedCard = ref<Card | null>(null)
const viewedCard = ref<Card | null>(null)
const showNickname = ref(false)
const showLink = ref(false)
const showPrivacy = ref(false)
const showShop = ref(false)
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
  state.value ? effChance(state.value.chance, risk.value, state.value.charmLevel) : 0,
)
const displayGain = computed(() => gainFor(displayChance.value, risk.value))

function setRisk(lvl: number) {
  risk.value = lvl
  play('switch')
}

// lockout after each roll so results land with suspense instead of spam clicks
const CLICK_COOLDOWN_MS = 800

async function onPress(center: { x: number; y: number }) {
  if (busy.value) return
  busy.value = true
  const result = await click(risk.value)
  setTimeout(() => (busy.value = false), CLICK_COOLDOWN_MS)
  if (!result) return

  if (result.success) {
    message.value = result.win ? t('win') : result.tierUp ? t('tierUp') : t('success')
    if (result.bonusClicks > 0) message.value += ` 🎟️+${result.bonusClicks}`
    messageColor.value = result.win ? 'text-yellow-300' : 'text-emerald-400'
    burst(center.x, center.y, [TIER_COLORS[result.tier], '#ffffff', '#facc15'], result.tierUp ? 120 : 60)
    play(result.win ? 'win' : `success_${result.tier}`)
    if (result.win) confetti()
  } else if (result.shieldUsed) {
    message.value = t('shieldSaved')
    messageColor.value = 'text-amber-300'
    play('switch')
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

onMounted(async () => {
  const step = () => loadProgress.value++
  const boot = Promise.all([
    preloadAudio(step, AUDIO_PRELOAD_TIMEOUT_MS),
    ...IMAGE_ASSETS.map((src) => preloadImage(src).then(step)),
    loadState().then(() => {
      step()
      if (state.value && !state.value.nickname) showNickname.value = true
    }),
    loadLeaderboard().then(step),
    loadCards().then(step),
  ])
  let lastHour = new Date().getHours()
  setInterval(() => {
    now.value = Date.now()
    const h = new Date().getHours()
    if (h !== lastHour) {
      lastHour = h
      if (state.value && state.value.quotaLeft <= 0) loadState()
    }
  }, 1000)
  // background tabs throttle the interval, so the timer freezes and a missed
  // hour rollover leaves stale quota — resync clock and server state on return
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible') return
    now.value = Date.now()
    lastHour = new Date().getHours()
    loadState()
  })
  await boot
  ready.value = true
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
                class="block w-full px-3 py-2 text-left text-slate-300 hover:bg-slate-800"
                @click="menuOpen = false; showLink = true"
              >
                🔗 {{ t('linkDevice') }}
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
          v-else-if="state"
          class="text-sm text-slate-400 hover:text-slate-200"
          @click="showNickname = true"
        >
          ✏️ {{ t('setName') }}
        </button>
        <button
          class="rounded-full border border-slate-600 px-3 py-1 text-xs font-bold text-slate-300 hover:bg-slate-800"
          @click="toggleLang(); play('switch')"
        >
          {{ lang === 'ko' ? 'EN' : '한국어' }}
        </button>
      </div>
    </header>

    <div v-if="!ready" class="flex flex-col items-center justify-center gap-4 py-32">
      <p class="animate-pulse text-4xl">🔘</p>
      <div class="h-2 w-48 overflow-hidden rounded-full bg-slate-800">
        <div
          class="h-full rounded-full bg-yellow-400 transition-all duration-200"
          :style="{ width: `${Math.round((loadProgress / loadTotal) * 100)}%` }"
        ></div>
      </div>
      <p class="text-xs tracking-widest text-slate-500">{{ t('loading') }}</p>
    </div>

    <main v-else class="mx-auto grid max-w-6xl gap-6 px-4 pb-12 lg:grid-cols-[280px_1fr_280px]">
      <Leaderboard class="order-2 lg:order-1" />

      <div class="order-1 flex flex-col items-center gap-5 pt-4 lg:order-2">
        <p class="text-sm text-slate-400">{{ t('subtitle') }}</p>

        <template v-if="state">
          <TierBadge :tier="state.tier" />
          <StarRow :stars="state.stars" />
          <span v-if="state.shieldCharges > 0" class="text-xs font-bold text-sky-300">
            🛡️×{{ state.shieldCharges }}
          </span>

          <TheButton
            :tier="state.tier"
            :chance="displayChance"
            :risky="risk > 0"
            :disabled="disabled"
            @press="onPress"
          />

          <p class="h-6 text-center font-bold" :class="messageColor">{{ message }}</p>

          <div class="flex flex-col items-center gap-1 select-none">
            <div class="flex items-center gap-2 whitespace-nowrap">
              <span class="text-sm font-bold" :class="risk ? 'text-rose-400' : 'text-slate-400'">
                🔥 {{ t('riskIt') }}
              </span>
              <div class="flex overflow-hidden rounded-full border border-slate-700">
                <button
                  v-for="lvl in MAX_RISK + 1"
                  :key="lvl - 1"
                  class="px-2.5 py-1 text-xs font-bold transition-colors sm:px-3"
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
            </div>
            <span class="h-4 text-xs whitespace-nowrap text-slate-500">
              <template v-if="risk">{{ t('chance') }} 1/{{ risk + 1 }} · ★+{{ displayGain }}</template>
            </span>
          </div>

          <button
            class="rounded-full border border-yellow-500/40 bg-yellow-400/10 px-4 py-1 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20"
            @click="showShop = true; play('switch')"
          >
            🛒 {{ t('shop') }} · 💰 {{ state.coins }}
          </button>

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

    <footer class="flex items-center justify-center gap-4 pb-6 text-slate-500">
      <a
        href="https://github.com/hwhang0917/the-button"
        target="_blank"
        rel="noopener"
        aria-label="GitHub"
        class="hover:text-slate-300"
      >
        <svg viewBox="0 0 24 24" fill="currentColor" class="h-5 w-5" aria-hidden="true">
          <path
            d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"
          />
        </svg>
      </a>
      <button class="text-xs underline hover:text-slate-300" @click="showPrivacy = true">
        {{ t('privacy') }}
      </button>
    </footer>

    <img
      v-if="ready && state && state.stars >= 13"
      src="/stich.gif"
      alt="pet"
      class="pet-bounce fixed bottom-4 right-4 z-20 h-24 w-24 object-contain drop-shadow-[0_0_15px_#a78bfa]"
    />

    <CardReveal v-if="droppedCard" :card="droppedCard" @close="droppedCard = null" />
    <CardReveal v-if="viewedCard" :card="viewedCard" :drop="false" @close="viewedCard = null" />
    <NicknameModal
      v-if="ready && showNickname"
      @close="showNickname = false"
      @link="showNickname = false; showLink = true"
    />
    <LinkModal v-if="showLink" @close="showLink = false" />
    <ShopModal v-if="showShop" @close="showShop = false" />
    <div
      v-if="showPrivacy"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
      @click.self="showPrivacy = false"
    >
      <div class="flex max-h-[80vh] w-full max-w-md flex-col gap-4 overflow-y-auto rounded-xl border border-slate-700 bg-slate-900 p-6">
        <h2 class="text-center text-lg font-bold text-slate-100">🔒 {{ t('privacy') }}</h2>
        <p class="whitespace-pre-line text-sm leading-relaxed text-slate-300">{{ t('privacyBody') }}</p>
        <button
          class="rounded-lg border border-slate-600 py-2 text-sm text-slate-300 hover:bg-slate-800"
          @click="showPrivacy = false"
        >
          OK
        </button>
      </div>
    </div>
  </div>
</template>
