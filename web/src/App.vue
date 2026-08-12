<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  armTalisman,
  cancelTalisman,
  cards,
  click,
  defuseCard,
  deletePlayer,
  effChance,
  fuseCards,
  gainFor,
  loadCards,
  loadLeaderboard,
  loadState,
  nextRarity,
  offline,
  prevRarity,
  prestigeReward,
  prestigeStreak,
  startHealthCheck,
  state,
  talismanBonus,
  type Card,
} from './useGame'
import { defineAsyncComponent, nextTick, watch } from 'vue'
import { driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { t, lang, toggleLang } from './i18n'
import { play, preloadAudio, soundCount, vibrate } from './audio'
import { burst, confetti } from './particles'
import { COIN_COLORS, useCoinCounter } from './useCoinCounter'
import { TIER_COLORS, type Rarity, type Tier } from './tiers'
import { cardName } from './cards'
import { cfg, loadConfig } from './config'
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
import TalismanPicker from './components/TalismanPicker.vue'
import ConfirmModal from './components/ConfirmModal.vue'

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
// order matches the t('tutorial') steps
const TUT_SELECTORS = [
  '#tut-button',
  '#tut-risk',
  '#tut-quota',
  '#tut-shop',
  '#tut-shop-sell',
  '#tut-shop-lottery',
  '#tut-shop-pack',
  '#tut-shop-refill',
  '#tut-shop-charm',
  '#tut-shop-stamina',
  '#tut-shop-headstart',
  '#tut-collection',
  '#tut-rank',
]
// which drawer holds a step's target on phones; the rest sit in the main column
const TUT_PANEL: Record<string, 'rank' | 'collection'> = {
  '#tut-collection': 'collection',
  '#tut-rank': 'rank',
}

function startTutorial() {
  localStorage.setItem(TUTORIAL_SEEN_KEY, '1')
  const steps = t('tutorial')
  // driver measures a target the moment it highlights it, so each step first
  // puts the UI into the state that target needs — the shop modal for the shop
  // rows, the right drawer on phones — and only then advances. Both are derived
  // from the selector, so reordering the tour cannot desync them.
  const goto = (i: number, move: () => void) => async () => {
    const sel = TUT_SELECTORS[i] ?? ''
    showShop.value = sel.startsWith('#tut-shop-')
    panel.value = drawerVisible() ? (TUT_PANEL[sel] ?? '') : ''
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
      panel.value = ''
    },
    steps: TUT_SELECTORS.map((element, i) => ({
      element,
      popover: {
        title: steps[i].title,
        description: steps[i].desc,
        onNextClick: goto(i + 1, () => d.moveNext()),
        onPrevClick: goto(i - 1, () => d.movePrevious()),
      },
    })),
  })
  goto(0, () => d.drive())()
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
    play('talisman')
    vibrate([10, 20, 25]) // charge-up tick as the talisman locks in
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

const cancelTalismanAsk = ref(false)
// red pulse on the 🃏 chip as a card burns to save the streak
const saveFlash = ref(false)

async function confirmCancelTalisman() {
  cancelTalismanAsk.value = false
  if (await cancelTalisman()) play('switch')
}

// lossy on purpose: fusion cost 3, defusion returns 2 — warn before burning
const defuseAsk = ref<{ tier: Tier; rarity: Rarity; lower: Rarity } | null>(null)

function onDefuse() {
  const v = viewedCard.value
  const lower = v && prevRarity(v.rarity)
  if (!v || !lower) return
  defuseAsk.value = { tier: v.tier, rarity: v.rarity, lower }
}

async function confirmDefuse() {
  const d = defuseAsk.value
  defuseAsk.value = null
  if (!d) return
  if (await defuseCard(d.tier, d.rarity)) {
    play('switch')
    vibrate([25, 20, 15]) // decaying pulse: something broke apart
    viewedCard.value = { tier: d.tier, rarity: d.lower }
  }
}

async function onFuse() {
  const v = viewedCard.value
  if (!v) return
  const next = nextRarity(v.rarity)
  if (!next || !(await fuseCards(v.tier, v.rarity))) return
  play('success_gold')
  const fx = RARITY_BURST[next]
  burst(window.innerWidth / 2, window.innerHeight / 2 - 40, fx.colors, fx.count)
  // fusion rumble grows with the result rarity
  vibrate(next === 'prismatic' ? [40, 30, 80, 30, 120] : next === 'holo' ? [30, 30, 60] : [20, 30, 40])
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
const showTalismanPick = ref(false)
const tutorialPending = ref(!localStorage.getItem(TUTORIAL_SEEN_KEY))

// first visit: run the tour once the game is ready and the nickname modal is out of the way
watch([ready, showNickname], async () => {
  if (!ready.value || showNickname.value || !tutorialPending.value) return
  tutorialPending.value = false
  await nextTick() // the tour targets live inside the v-else main
  startTutorial()
})
const menuOpen = ref(false)

// The leaderboard and the collection are reference material, not part of the
// loop, so on phones they slide in from the right instead of stacking below the
// button and forcing a scroll. One <aside> serves both roles: a fixed drawer up
// to lg, a plain grid column from lg up.
const panel = ref<'' | 'rank' | 'collection'>('')
const desktop = matchMedia('(min-width: 1024px)')
const drawerVisible = () => !desktop.matches
// resizing up turns the drawer back into a static column, so drop the state or
// the body scroll lock would stay stuck on a desktop-width page
desktop.addEventListener('change', (e) => {
  if (e.matches) panel.value = ''
})
const modalOpen = computed(() =>
  Boolean(
    droppedCard.value ||
      viewedCard.value ||
      showNickname.value ||
      showLink.value ||
      showShop.value ||
      showPrivacy.value ||
      showOdds.value ||
      showTalismanPick.value ||
      panel.value,
  ),
)
// modals cover the page; freeze the body so the background can't scroll under them
watch(modalOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})

const shownCoins = useCoinCounter(() => state.value?.coins ?? 0)

const nextPrestigeReward = computed(() =>
  state.value ? prestigeReward(state.value.prestige) : 0,
)

async function onPrestige() {
  const gained = await prestigeStreak()
  if (gained === null) return
  message.value = `${t('prestigeDone')} +${gained}💰`
  messageColor.value = 'text-fuchsia-300'
  play('win')
  confetti()
  burst(window.innerWidth / 2, window.innerHeight / 2, COIN_COLORS, 80)
}

const deleteAsk = ref(false)

async function confirmDelete() {
  deleteAsk.value = false
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
// base chance (risk + charm); the talisman bonus is shown separately as +N%
// and never feeds the payout, so displayGain stays correct deriving from this
const displayChance = computed(() =>
  state.value ? effChance(state.value.chance, risk.value, state.value.charmLevel) : 0,
)
const displayBonus = computed(() => (state.value ? talismanBonus(state.value) : 0))
const displayGain = computed(() => gainFor(displayChance.value, risk.value))

function setRisk(lvl: number) {
  risk.value = lvl
  play('switch')
  vibrate(4 + lvl * 6) // buzz escalates with the risk you're signing up for
}

// `busy` already covers the whole in-flight request, so this is only a short
// debounce against a double-fire on release — keep it well under the fail shake
// (500ms) so the button never feels like it is holding you back
const CLICK_COOLDOWN_MS = 150

async function onPress(center: { x: number; y: number }) {
  if (busy.value) return
  busy.value = true
  const starsBefore = state.value?.stars ?? 0
  const result = await click(risk.value)
  setTimeout(() => (busy.value = false), CLICK_COOLDOWN_MS)
  if (!result) return

  if (result.success) {
    message.value = result.win
      ? t('win').replace('{n}', String(state.value?.maxStars ?? 15))
      : result.tierUp
        ? t('tierUp')
        : t('success')
    if (result.bonusClicks > 0) message.value += ` 🎟️+${result.bonusClicks}`
    if (result.jackpot > 0) {
      message.value += ` 💰+${result.jackpot}`
      burst(center.x, center.y - 60, COIN_COLORS, 80)
    }
    messageColor.value = result.win ? 'text-yellow-300' : 'text-emerald-400'
    burst(center.x, center.y, [TIER_COLORS[result.tier], '#ffffff', '#facc15'], result.tierUp ? 120 : 60)
    play(result.win ? 'win' : `success_${result.tier}`)
    if (result.tierUp && !result.win) vibrate([30, 30, 70]) // richer than the plain success buzz
    if (result.win) confetti()
  } else if (result.talismanUsed && result.stars > 0 && result.stars === starsBefore) {
    message.value = t('talismanSaved')
    messageColor.value = 'text-amber-300'
    play('shield')
    vibrate([30, 40, 60]) // "phew" double-pulse for a save
    saveFlash.value = true
    setTimeout(() => (saveFlash.value = false), 900)
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
  // config first: prices, odds and card effects all read from it, and the
  // whole <main> renders only once `ready` flips, so nothing reads it early
  await loadConfig()
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
  startHealthCheck()
  await boot
  ready.value = true
})
</script>

<template>
  <div :class="{ shake: shaking }" class="min-h-screen text-slate-200">
    <div v-if="flashing" class="flash-red pointer-events-none fixed inset-0 z-30 bg-rose-600"></div>

    <div
      v-if="state?.devMode"
      class="fixed inset-x-0 top-0 z-50 bg-amber-400 py-0.5 text-center text-[11px] font-black tracking-widest text-slate-900"
    >
      ⚠ DEV MODE — 100% SUCCESS
    </div>

    <div
      v-if="offline"
      class="fixed inset-x-0 top-0 z-50 flex items-center justify-center gap-2 bg-rose-600 py-1 text-center text-xs font-bold text-white"
    >
      <span class="inline-block h-3 w-3 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
      {{ t('offline') }}
    </div>

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
                @click="menuOpen = false; deleteAsk = true"
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

    <main v-else class="mx-auto grid max-w-6xl gap-4 px-4 pb-2 sm:gap-6 sm:pb-12 lg:grid-cols-[280px_1fr_280px]">
      <!-- one shared backdrop for whichever drawer is open -->
      <div
        v-if="panel"
        class="fixed inset-0 z-20 bg-black/60 backdrop-blur-sm lg:hidden"
        @click="panel = ''"
      ></div>

      <aside class="drawer lg:order-1" :class="{ open: panel === 'rank' }">
        <button
          class="absolute right-3 top-3 flex h-8 w-8 items-center justify-center rounded-full border border-slate-700 text-slate-400 hover:bg-slate-800 hover:text-slate-100 lg:hidden"
          :aria-label="t('later')"
          @click="panel = ''"
        >
          ✕
        </button>
        <Leaderboard id="tut-rank" />

      </aside>

      <div class="order-1 flex flex-col items-center gap-2 pt-1 sm:gap-4 sm:pt-4 lg:order-2">
        <p class="hidden text-sm text-slate-400 sm:block">{{ t('subtitle') }}</p>

        <template v-if="state">
          <!-- one compact status row keeps the core info above the fold on phones -->
          <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
            <TierBadge :tier="state.tier" />
            <StarRow :stars="state.stars" :prestige="state.prestige" :max="state.maxStars" />
            <!-- the talisman slot sits at TierBadge's size so the row reads as
                 one system, and stays a comfortable tap target on phones -->
            <button
              v-if="state.talismanTier"
              class="inline-flex items-center gap-1.5 rounded-full px-4 py-1 text-sm font-bold hover:opacity-80"
              :class="{ 'shield-hit': saveFlash }"
              :style="{
                color: TIER_COLORS[state.talismanTier],
                border: `1px solid ${TIER_COLORS[state.talismanTier]}66`,
                background: `${TIER_COLORS[state.talismanTier]}1a`,
              }"
              :title="t('talismanCancelConfirm')"
              @click="cancelTalismanAsk = true"
            >
              🃏 {{ cardName(state.talismanTier, state.talismanRarity as Rarity) }}
              <span class="text-slate-400">✕</span>
            </button>
            <button
              v-else
              class="inline-flex items-center gap-1.5 rounded-full border border-dashed border-amber-400/50 bg-amber-400/5 px-4 py-1 text-sm font-bold text-amber-300/80 hover:border-amber-400 hover:bg-amber-400/15 hover:text-amber-200"
              :title="t('talismanPick')"
              @click="showTalismanPick = true; play('switch')"
            >
              🃏 {{ t('talismanSlotEmpty') }}
            </button>
          </div>

          <div id="tut-button" class="-my-3 sm:my-0">
            <TheButton
              :tier="state.tier"
              :chance="displayChance"
              :bonus="displayBonus"
              :risk="risk"
              :max-risk="cfg().maxRisk"
              :pressure="state.stars / state.maxStars"
              :disabled="disabled"
              :prestige="state.prestige"
              :talisman="!!state.talismanTier"
              @press="onPress"
            />
          </div>

          <p class="h-5 text-center text-sm font-bold sm:h-6 sm:text-base" :class="messageColor">{{ message }}</p>

          <button
            v-if="state.win"
            class="animate-pulse rounded-full border-2 border-fuchsia-400 bg-fuchsia-500/20 px-8 py-3 text-lg font-black tracking-widest text-fuchsia-200 hover:bg-fuchsia-500/30"
            @click="onPrestige"
          >
            ✨ {{ t('prestige') }} +{{ nextPrestigeReward }}💰
          </button>

          <div id="tut-risk" class="flex flex-col items-center gap-1 select-none">
            <div class="flex items-center gap-2 whitespace-nowrap">
              <span class="text-sm font-bold" :class="risk ? 'text-rose-400' : 'text-slate-400'">
                🔥 {{ t('riskIt') }}
              </span>
              <div class="flex overflow-hidden rounded-full border border-slate-700">
                <button
                  v-for="lvl in cfg().maxRisk + 1"
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

          <!-- shop, quota, and best share one wrapping row instead of three -->
          <div class="flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
            <button
              id="tut-shop"
              class="rounded-full border border-yellow-500/40 bg-yellow-400/10 px-4 py-1 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20"
              @click="showShop = true; play('switch')"
            >
              🛒 {{ t('shop') }} · 💰 {{ shownCoins }}
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
          </div>

          <!-- phones: the two reference panels live in drawers, so the core
               loop above fits without scrolling. lg lays them out as columns
               and these triggers disappear. -->
          <div class="flex items-center gap-2 lg:hidden">
            <button
              class="rounded-full border border-slate-700 bg-slate-800/60 px-4 py-1 text-xs font-bold text-slate-300 hover:border-slate-500 hover:text-slate-100"
              @click="panel = 'rank'; play('switch')"
            >
              🏆 {{ t('leaderboard') }}
            </button>
            <button
              class="rounded-full border border-slate-700 bg-slate-800/60 px-4 py-1 text-xs font-bold text-slate-300 hover:border-slate-500 hover:text-slate-100"
              @click="panel = 'collection'; play('switch')"
            >
              🃏 {{ t('collection') }}
            </button>
          </div>
        </template>
      </div>

      <aside class="drawer lg:order-3" :class="{ open: panel === 'collection' }">
        <button
          class="absolute right-3 top-3 flex h-8 w-8 items-center justify-center rounded-full border border-slate-700 text-slate-400 hover:bg-slate-800 hover:text-slate-100 lg:hidden"
          :aria-label="t('later')"
          @click="panel = ''"
        >
          ✕
        </button>
        <CardCollection id="tut-collection" :key="cards.length" @view="viewedCard = $event" />
      </aside>
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
      <button class="text-xs underline hover:text-slate-300" @click="showOdds = true; play('switch')">
        {{ t('oddsLink') }}
      </button>
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
      @defuse="onDefuse"
    />
    <NicknameModal
      v-if="ready && showNickname"
      @close="showNickname = false"
      @link="showNickname = false; linkViaNickname = true; showLink = true"
    />
    <LinkModal v-if="showLink" @close="closeLink" />
    <ShopModal v-if="showShop" @close="showShop = false" />
    <OddsModal v-if="showOdds" @close="showOdds = false" />
    <TalismanPicker v-if="showTalismanPick" @close="showTalismanPick = false" />
    <ConfirmModal
      v-if="defuseAsk"
      :title="`⚠️ ${t('defuse')}`"
      :message="t('defuseConfirm').replace('{rarity}', t('rarity')[defuseAsk.lower])"
      :confirm-label="t('defuse')"
      :cancel-label="t('later')"
      @confirm="confirmDefuse"
      @cancel="defuseAsk = null"
    />
    <ConfirmModal
      v-if="cancelTalismanAsk"
      :title="`🃏 ${t('disarm')}`"
      :message="t('talismanCancelConfirm')"
      :confirm-label="t('disarm')"
      :cancel-label="t('later')"
      @confirm="confirmCancelTalisman"
      @cancel="cancelTalismanAsk = false"
    />
    <ConfirmModal
      v-if="deleteAsk"
      :title="`🗑️ ${t('deleteData')}`"
      :message="t('deleteConfirm')"
      :confirm-label="t('deleteData')"
      :cancel-label="t('later')"
      @confirm="confirmDelete"
      @cancel="deleteAsk = false"
    />
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
