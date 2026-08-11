<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  armTalisman,
  cards,
  click,
  deletePlayer,
  effChance,
  fuseCards,
  gainFor,
  loadCards,
  loadLeaderboard,
  loadState,
  MAX_RISK,
  nextRarity,
  PRESTIGE_REWARDS,
  prestigeStreak,
  state,
  talismanBonus,
  type Card,
} from './useGame'
import { defineAsyncComponent, nextTick, watch } from 'vue'
import { driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { t, lang, toggleLang } from './i18n'
import { play, preloadAudio, soundCount } from './audio'
import { burst, confetti } from './particles'
import { TIER_COLORS, type Rarity } from './tiers'
import TheButton from './components/TheButton.vue'
import StarRow from './components/StarRow.vue'
import TierBadge from './components/TierBadge.vue'
import Leaderboard from './components/Leaderboard.vue'
import CardCollection from './components/CardCollection.vue'
import CardReveal from './components/CardReveal.vue'
import NicknameModal from './components/NicknameModal.vue'
import LinkModal from './components/LinkModal.vue'
import ShopModal from './components/ShopModal.vue'
// katex is heavy: load it only when the odds popup is actually opened
const OddsModal = defineAsyncComponent(() => import('./components/OddsModal.vue'))

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

const TUTORIAL_SEEN_KEY = 'bt_tutorial_seen'
// order matches the t('tutorial') steps; indexes 4-8 live inside the shop modal
const TUT_SELECTORS = [
  '#tut-button',
  '#tut-risk',
  '#tut-quota',
  '#tut-shop',
  '#tut-shop-sell',
  '#tut-shop-lottery',
  '#tut-shop-shield',
  '#tut-shop-charm',
  '#tut-shop-headstart',
  '#tut-collection',
  '#tut-rank',
]
const TUT_SHOP_FIRST = 4
const TUT_SHOP_LAST = 8

function startTutorial() {
  localStorage.setItem(TUTORIAL_SEEN_KEY, '1')
  const steps = t('tutorial')
  // the in-shop steps need the modal mounted before they can be highlighted,
  // so the boundary steps swap the modal in/out and then advance manually
  const swapShop = (open: boolean, move: () => void) => async () => {
    showShop.value = open
    await nextTick()
    move()
  }
  const d = driver({
    showProgress: true,
    nextBtnText: t('tutNext'),
    prevBtnText: t('tutPrev'),
    doneBtnText: t('tutDone'),
    onDestroyed: () => {
      showShop.value = false
    },
    steps: TUT_SELECTORS.map((element, i) => ({
      element,
      popover: {
        title: steps[i].title,
        description: steps[i].desc,
        ...(i === TUT_SHOP_FIRST - 1 && { onNextClick: swapShop(true, () => d.moveNext()) }),
        ...(i === TUT_SHOP_FIRST && { onPrevClick: swapShop(false, () => d.movePrevious()) }),
        ...(i === TUT_SHOP_LAST && { onNextClick: swapShop(false, () => d.moveNext()) }),
        ...(i === TUT_SHOP_LAST + 1 && { onPrevClick: swapShop(true, () => d.movePrevious()) }),
      },
    })),
  })
  d.drive()
}

const risk = ref(0)
const busy = ref(false)
const shaking = ref(false)
const flashing = ref(false)
const message = ref('')
const messageColor = ref('text-slate-300')
const droppedCard = ref<Card | null>(null)
const viewedCard = ref<Card | null>(null)
const viewedCount = computed(() => {
  const v = viewedCard.value
  if (!v) return 0
  return cards.value.find((c) => c.tier === v.tier && c.rarity === v.rarity)?.count ?? 0
})

async function onArm() {
  const v = viewedCard.value
  if (!v) return
  if (await armTalisman(v.tier, v.rarity)) {
    play('switch')
    viewedCard.value = null
  }
}

// fusion celebration: burst colors and particle count follow the RESULT rarity
const RARITY_BURST: Record<Rarity, { colors: string[]; count: number }> = {
  common: { colors: ['#e2e8f0', '#cbd5e1'], count: 40 },
  rare: { colors: ['#7dd3fc', '#38bdf8', '#0ea5e9', '#ffffff'], count: 60 },
  holo: { colors: ['#f0abfc', '#fcd34d', '#22d3ee', '#a78bfa', '#ffffff'], count: 90 },
  prismatic: { colors: ['#ff0084', '#fcff00', '#00fff0', '#7c00ff', '#ffffff'], count: 140 },
}

async function onFuse() {
  const v = viewedCard.value
  if (!v) return
  const next = nextRarity(v.rarity)
  if (!next || !(await fuseCards(v.tier, v.rarity))) return
  play('success_gold')
  const fx = RARITY_BURST[next]
  burst(window.innerWidth / 2, window.innerHeight / 2 - 40, fx.colors, fx.count)
  // flip the popup to the freshly fused card so its rarity effect shows
  viewedCard.value = { tier: v.tier, rarity: next }
}
const showNickname = ref(false)
const showLink = ref(false)
// link modal reached from the first-visit nickname prompt: closing it without
// claiming must fall back to the prompt, or the player ends up unnamed
const linkViaNickname = ref(false)

function closeLink() {
  showLink.value = false
  if (linkViaNickname.value) {
    linkViaNickname.value = false
    showNickname.value = true
  }
}
const showPrivacy = ref(false)
const showShop = ref(false)
const showOdds = ref(false)
const tutorialPending = ref(!localStorage.getItem(TUTORIAL_SEEN_KEY))

// first visit: run the tour once the game is ready and the nickname modal is out of the way
watch([ready, showNickname], async () => {
  if (!ready.value || showNickname.value || !tutorialPending.value) return
  tutorialPending.value = false
  await nextTick() // the tour targets live inside the v-else main
  startTutorial()
})
const menuOpen = ref(false)

const modalOpen = computed(() =>
  Boolean(
    droppedCard.value ||
      viewedCard.value ||
      showNickname.value ||
      showLink.value ||
      showShop.value ||
      showPrivacy.value ||
      showOdds.value,
  ),
)
// modals cover the page; freeze the body so the background can't scroll under them
watch(modalOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})

const prestigeReward = computed(() =>
  state.value ? PRESTIGE_REWARDS[Math.min(state.value.prestige, PRESTIGE_REWARDS.length - 1)] : 0,
)

async function onPrestige() {
  const gained = await prestigeStreak()
  if (gained === null) return
  message.value = `${t('prestigeDone')} +${gained}💰`
  messageColor.value = 'text-fuchsia-300'
  play('win')
  confetti()
}

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
// include the armed talisman's bonus when it would actually fire (tier match),
// so the button shows the same P the server will roll
const displayChance = computed(() => {
  if (!state.value) return 0
  const base = effChance(state.value.chance, risk.value, state.value.charmLevel)
  return Math.min(100, base + talismanBonus(state.value))
})
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
  const starsBefore = state.value?.stars ?? 0
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
  } else if (!result.success && result.talismanUsed && result.stars === starsBefore && starsBefore > 0) {
    message.value = t('talismanSaved')
    messageColor.value = 'text-amber-300'
    play('switch')
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
          class="flex h-7 w-7 items-center justify-center rounded-full border border-slate-600 text-xs font-bold text-slate-300 hover:bg-slate-800"
          aria-label="tutorial"
          @click="startTutorial(); play('switch')"
        >
          ?
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
      <Leaderboard id="tut-rank" class="order-2 lg:order-1" />

      <div class="order-1 flex flex-col items-center gap-5 pt-4 lg:order-2">
        <p class="text-sm text-slate-400">{{ t('subtitle') }}</p>

        <template v-if="state">
          <TierBadge :tier="state.tier" />
          <StarRow :stars="state.stars" :prestige="state.prestige" />
          <span v-if="state.shieldCharges > 0" class="text-xs font-bold text-sky-300">
            🛡️×{{ state.shieldCharges }}
          </span>
          <span
            v-if="state.talismanTier"
            class="text-xs font-bold"
            :style="{ color: TIER_COLORS[state.talismanTier] }"
          >
            🃏 {{ t('tier')[state.talismanTier] }}·{{ t('rarity')[state.talismanRarity as Rarity] }}
          </span>

          <div id="tut-button" class="relative">
            <button
              class="absolute right-1 top-1 z-10 flex h-6 w-6 items-center justify-center rounded-full border border-slate-600 bg-slate-900/70 text-[10px] font-bold text-slate-400 hover:text-slate-200"
              :aria-label="t('oddsTitle')"
              @click="showOdds = true; play('switch')"
            >
              ℹ
            </button>
            <TheButton
              :tier="state.tier"
              :chance="displayChance"
              :risky="risk > 0"
              :disabled="disabled"
              :prestige="state.prestige"
              :shield="state.shieldCharges > 0"
              :talisman="!!state.talismanTier"
              @press="onPress"
            />
          </div>

          <p class="h-6 text-center font-bold" :class="messageColor">{{ message }}</p>

          <button
            v-if="state.win"
            class="animate-pulse rounded-full border-2 border-fuchsia-400 bg-fuchsia-500/20 px-8 py-3 text-lg font-black tracking-widest text-fuchsia-200 hover:bg-fuchsia-500/30"
            @click="onPrestige"
          >
            ✨ {{ t('prestige') }} +{{ prestigeReward }}💰
          </button>

          <div id="tut-risk" class="flex flex-col items-center gap-1 select-none">
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
            id="tut-shop"
            class="rounded-full border border-yellow-500/40 bg-yellow-400/10 px-4 py-1 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20"
            @click="showShop = true; play('switch')"
          >
            🛒 {{ t('shop') }} · 💰 {{ state.coins }}
          </button>

          <p id="tut-quota" class="text-sm text-slate-400">
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

      <CardCollection id="tut-collection" class="order-3" :key="cards.length" @view="viewedCard = $event" />
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
    <CardReveal
      v-if="viewedCard"
      :key="viewedCard.tier + viewedCard.rarity"
      :card="viewedCard"
      :drop="false"
      :count="viewedCount"
      @close="viewedCard = null"
      @arm="onArm"
      @fuse="onFuse"
    />
    <NicknameModal
      v-if="ready && showNickname"
      @close="showNickname = false"
      @link="showNickname = false; linkViaNickname = true; showLink = true"
    />
    <LinkModal v-if="showLink" @close="closeLink" />
    <ShopModal v-if="showShop" @close="showShop = false" />
    <OddsModal v-if="showOdds" @close="showOdds = false" />
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
