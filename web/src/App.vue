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
  refillAt,
  sellCard,
  sellStreak,
  startHealthCheck,
  state,
  streakValue,
  talismanBonus,
  type Card,
} from './useGame'
import { defineAsyncComponent, nextTick, watch } from 'vue'
import { driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { t, lang, toggleLang } from './i18n'
import { theme, toggleTheme } from './theme'
import { play, preloadAudio, soundCount, vibrate } from './audio'
import { burst, confetti } from './particles'
import { COIN_COLORS, useCoinCounter } from './useCoinCounter'
import { TIER_COLORS, type Rarity, type Tier } from './tiers'
import { cardName, effectFor } from './cards'
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

const IMAGE_ASSETS = ['/wallpaper.jpg', '/star.png']
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
  '#tut-shop-magnet',
  '#tut-shop-golden',
  '#tut-collection',
  '#tut-rank',
]
// which drawer holds a step's target on phones; the rest sit in the main column
const TUT_PANEL: Record<string, 'rank' | 'collection'> = {
  '#tut-collection': 'collection',
  '#tut-rank': 'rank',
}
// which shop modal holds a step's target (streak sell lives on the main screen)
const TUT_SHOP: Record<string, 'items' | 'skills'> = {
  '#tut-shop-lottery': 'items',
  '#tut-shop-pack': 'items',
  '#tut-shop-refill': 'items',
  '#tut-shop-charm': 'skills',
  '#tut-shop-stamina': 'skills',
  '#tut-shop-headstart': 'skills',
  '#tut-shop-magnet': 'skills',
  '#tut-shop-golden': 'skills',
}

function startTutorial() {
  localStorage.setItem(TUTORIAL_SEEN_KEY, '1')
  const steps = t('tutorial')
  // desktops get a closing keyboard-shortcuts step (elementless = centered
  // popover); phones have no keyboard, so their tour ends one step earlier
  const sels = drawerVisible() ? TUT_SELECTORS : [...TUT_SELECTORS, '']
  // driver measures a target the moment it highlights it, so each step first
  // puts the UI into the state that target needs — the shop modal for the shop
  // rows, the right drawer on phones — and only then advances. Both are derived
  // from the selector, so reordering the tour cannot desync them.
  const goto = (i: number, move: () => void) => async () => {
    const sel = sels[i] ?? ''
    showShop.value = TUT_SHOP[sel] ?? ''
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
      showShop.value = ''
      panel.value = ''
    },
    steps: sels.map((element, i) => ({
      element: element || undefined,
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

// selling the last copy empties the collection slot, so only that asks first
const sellCardAsk = ref<{ tier: Tier; rarity: Rarity } | null>(null)

function onSellCard() {
  const v = viewedCard.value
  if (!v) return
  if (viewedCount.value === 1) sellCardAsk.value = { tier: v.tier, rarity: v.rarity }
  else doSellCard(v.tier, v.rarity)
}

async function confirmSellCard() {
  const d = sellCardAsk.value
  sellCardAsk.value = null
  if (d) await doSellCard(d.tier, d.rarity)
}

async function doSellCard(tier: Tier, rarity: Rarity) {
  const gained = await sellCard(tier, rarity)
  if (gained === null) return
  play('streak-sell')
  vibrate([15, 20, 30])
  burst(window.innerWidth / 2, window.innerHeight / 2 - 40, COIN_COLORS, Math.min(80, 20 + gained))
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
const showShop = ref<'' | 'items' | 'skills'>('')
const sellAsk = ref(false)
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
// mobile hamburger drawer; lg+ shows everything inline and never needs it
const navOpen = ref(false)
function navTo(action: () => void) {
  navOpen.value = false
  play('switch')
  action()
}

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
      sellAsk.value ||
      navOpen.value ||
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
const coinEl = ref<HTMLElement | null>(null)

const streakSellValue = computed(() =>
  state.value ? streakValue(state.value.stars, state.value.headstartLevel) : 0,
)

async function confirmSellStreak() {
  sellAsk.value = false
  const gained = await sellStreak()
  if (gained) {
    play('streak-sell')
    vibrate([15, 20, 35]) // coins clattering in
    const r = coinEl.value?.getBoundingClientRect()
    // more coins, bigger shower
    if (r) burst(r.left + r.width / 2, r.top + r.height / 2, COIN_COLORS, Math.min(30 + Math.floor(gained / 2), 90))
  }
}

const nextPrestigeReward = computed(() =>
  state.value ? prestigeReward(state.value.prestige) : 0,
)

async function onPrestige() {
  const gained = await prestigeStreak()
  if (gained === null) return
  message.value = `${t('prestigeDone')} +${gained}💰`
  messageColor.value = 'text-fuchsia-300 light:text-fuchsia-700'
  play('win')
  // double wave so the promotion reads as a real celebration
  confetti()
  setTimeout(confetti, 450)
  burst(window.innerWidth / 2, window.innerHeight / 2, COIN_COLORS, 80)
}

const deleteAsk = ref(false)

async function confirmDelete() {
  deleteAsk.value = false
  if (await deletePlayer()) location.reload()
}

const now = ref(Date.now())

// counts down to the server-published bucket deadline (refillAt), not the
// client's top-of-hour — the clocks disagree by whatever the skew is
const refillIn = computed(() => {
  const s = Math.min(3599, Math.max(0, Math.ceil((refillAt.value - now.value) / 1000)))
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
const armedEffect = computed(() =>
  state.value?.talismanTier && state.value.talismanRarity
    ? effectFor(state.value.talismanTier, state.value.talismanRarity)
    : {},
)
// stars on success, mirroring Resolve's success branch: the armed card's chance
// bonus feeds only the roll, but maxRisk/guarantee/mult/bonus all shape the payout
const displayGain = computed(() => {
  const s = state.value
  if (!s) return 1
  const e = armedEffect.value
  const payRisk = e.maxRisk ? cfg().maxRisk : risk.value
  const base = e.guarantee ? 1 : gainFor(effChance(s.chance, payRisk, s.charmLevel), payRisk)
  return base * Math.max(1, e.mult ?? 0) + (e.bonus ?? 0)
})
// the clicks-left counter drains toward a warning: the last 30% goes orange,
// the last 10% red — relative to quota so stamina levels keep scale
const quotaColor = computed(() => {
  const s = state.value
  if (!s || s.quota <= 0) return 'text-slate-200'
  const ratio = s.quotaLeft / s.quota
  if (ratio <= 0.1) return 'text-rose-400 light:text-rose-600'
  if (ratio <= 0.3) return 'text-orange-400 light:text-orange-600'
  return 'text-slate-200'
})

// coins on success — Resolve's jackpot: overflow past the cap plus 황금손's per-star pay
const displayCoins = computed(() => {
  const s = state.value
  if (!s) return 0
  const overflow = Math.max(0, s.stars + displayGain.value - s.maxStars)
  const newStars = Math.min(s.stars + displayGain.value, s.maxStars)
  return cfg().overflowCoinPer * overflow + (armedEffect.value.coinWin ?? 0) * newStars
})

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
    messageColor.value = result.win
      ? 'text-yellow-300 light:text-yellow-600'
      : 'text-emerald-400 light:text-emerald-600'
    burst(center.x, center.y, [TIER_COLORS[result.tier], '#ffffff', '#facc15'], result.tierUp ? 120 : 60)
    play(result.win ? 'win' : `success_${result.tier}`)
    if (result.tierUp && !result.win) vibrate([30, 30, 70]) // richer than the plain success buzz
    if (result.win) confetti()
  } else if (result.saved) {
    message.value = t('talismanSaved')
    messageColor.value = 'text-amber-300 light:text-amber-700'
    play('shield')
    vibrate([30, 40, 60]) // "phew" double-pulse for a save
    saveFlash.value = true
    setTimeout(() => (saveFlash.value = false), 900)
  } else {
    message.value = t('fail')
    messageColor.value = 'text-rose-400 light:text-rose-600'
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

// the subtitle spot rotates through t('tips') — a random hop that never lands
// on the tip already on screen; :key remounts the <p> to replay the fade
const TIP_MS = 8000
const tip = ref(Math.floor(Math.random() * t('tips').length))
setInterval(() => {
  const n = t('tips').length
  tip.value = (tip.value + 1 + Math.floor(Math.random() * (n - 1))) % n
}, TIP_MS)

// keyboard shortcuts for the base screen: Space enchants, S opens the streak
// sale, I/K the item and skill shops, 1-4 pick the risk level, Esc backs out
// of what a shortcut opened. Ignored whenever another overlay could own the
// key instead: modals, main-screen confirms, the tutorial, or a form control.
function onKey(e: KeyboardEvent) {
  if (e.repeat || e.altKey || e.ctrlKey || e.metaKey) return
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
  if (e.key === 'Escape') {
    showShop.value = ''
    sellAsk.value = false
    navOpen.value = false
    return
  }
  if (document.querySelector('.driver-overlay')) return
  if (e.code === 'Space') {
    // inside an overlay Space means its confirming action — each overlay marks
    // that button with data-space, and the deepest-stacked one wins
    const confirmers = document.querySelectorAll<HTMLElement>('[data-space]')
    if (confirmers.length) {
      e.preventDefault()
      confirmers[confirmers.length - 1].click()
      return
    }
  }
  if (modalOpen.value || cancelTalismanAsk.value || menuOpen.value) return
  switch (e.code) {
    case 'Space':
      if (disabled.value) return
      // stops page scroll and keeps a previously-focused button from re-firing
      e.preventDefault()
      const r = document.getElementById('tut-button')?.getBoundingClientRect()
      onPress(r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : { x: innerWidth / 2, y: innerHeight / 2 })
      break
    case 'KeyS':
      // mirrors the sell button's disabled state
      if (streakSellValue.value && !state.value?.win) {
        sellAsk.value = true
        play('switch')
      }
      break
    case 'KeyI':
      showShop.value = 'items'
      play('switch')
      break
    case 'KeyK':
      showShop.value = 'skills'
      play('switch')
      break
    case 'Digit1':
    case 'Digit2':
    case 'Digit3':
    case 'Digit4': {
      const lvl = Number(e.code.slice(5)) - 1
      if (lvl <= cfg().maxRisk) setRisk(lvl)
      break
    }
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKey)
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
  // once the deadline passes with the quota still empty, ask the server until it
  // confirms the refill: a load that comes back too early (skew/latency) carries
  // the real remaining seconds, which re-arms refillAt; a failed fetch retries
  // via the fallback deadline set here
  const REFILL_RETRY_MS = 5000
  setInterval(() => {
    now.value = Date.now()
    if (state.value && state.value.quotaLeft <= 0 && now.value >= refillAt.value) {
      refillAt.value = now.value + REFILL_RETRY_MS
      loadState().catch(() => {})
    }
  }, 1000)
  // background tabs throttle the interval, so the timer freezes and a missed
  // rollover leaves stale quota — resync clock and server state on return
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible') return
    now.value = Date.now()
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
      class="fixed inset-x-0 top-0 z-50 bg-amber-400 py-0.5 text-center text-[11px] font-black tracking-widest text-black"
    >
      ⚠ DEV MODE — 100% SUCCESS
    </div>

    <!-- swallow every click/tap while offline: sits above modals (z-50),
         below the banner. checkHealth clears it the moment the server answers -->
    <div v-if="offline" class="fixed inset-0 z-[70] cursor-not-allowed bg-black/20"></div>
    <div
      v-if="offline"
      class="fixed inset-x-0 top-0 z-[80] flex items-center justify-center gap-2 bg-rose-600 py-1 text-center text-xs font-bold text-white"
    >
      <span class="inline-block h-3 w-3 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
      {{ t('offline') }}
    </div>

    <header class="flex items-center justify-between px-4 py-3 sm:px-8">
      <h1 class="text-xl font-black tracking-[0.2em] text-slate-100 sm:text-2xl">THE BUTTON</h1>
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
<!-- utilities stay inline on lg; phones reach them through the hamburger -->
        <button
          class="hidden h-7 w-7 items-center justify-center rounded-full border border-slate-600 text-xs font-bold text-slate-300 hover:bg-slate-800 lg:flex"
          aria-label="tutorial"
          @click="startTutorial(); play('switch')"
        >
          ?
        </button>
        <button
          class="hidden rounded-full border border-slate-600 px-3 py-1 text-xs font-bold text-slate-300 hover:bg-slate-800 lg:block"
          @click="toggleLang(); play('switch')"
        >
          {{ lang.toUpperCase() }}
        </button>
        <button
          class="hidden h-7 w-7 items-center justify-center rounded-full border border-slate-600 text-xs hover:bg-slate-800 lg:flex"
          :aria-label="theme === 'dark' ? 'light mode' : 'dark mode'"
          @click="toggleTheme(); play('switch')"
        >
          {{ theme === 'dark' ? '☀️' : '🌙' }}
        </button>
        <button
          class="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-600 text-lg text-slate-300 hover:bg-slate-800 lg:hidden"
          aria-label="menu"
          @click="navOpen = true; play('switch')"
        >
          ☰
        </button>
      </div>
    </header>

    <!-- mobile hamburger drawer -->
    <div v-if="navOpen" class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm lg:hidden" @click="navOpen = false">
      <nav
        class="nav-in absolute right-0 top-0 flex h-full w-64 flex-col gap-1.5 border-l border-slate-700 bg-slate-900 p-5"
        @click.stop
      >
        <button
          class="mb-2 self-end flex h-8 w-8 items-center justify-center rounded-full border border-slate-700 text-slate-400 hover:bg-slate-800 hover:text-slate-100"
          :aria-label="t('later')"
          @click="navOpen = false"
        >
          ✕
        </button>
        <button class="nav-item" @click="navTo(() => (panel = 'rank'))">🏆 {{ t('leaderboard') }}</button>
        <button class="nav-item" @click="navTo(() => (panel = 'collection'))">🃏 {{ t('collection') }}</button>
        <button class="nav-item" @click="navTo(() => (showShop = 'items'))">🎁 {{ t('itemShop') }}</button>
        <button class="nav-item" @click="navTo(() => (showShop = 'skills'))">📈 {{ t('skillShop') }}</button>
        <hr class="my-2 border-slate-700/60" />
        <button class="nav-item" @click="navTo(startTutorial)">❓ {{ t('menuTutorial') }}</button>
        <button class="nav-item" @click="navTo(toggleLang)">🌐 {{ lang.toUpperCase() }}</button>
        <button class="nav-item" @click="navTo(toggleTheme)">
          {{ theme === 'dark' ? `☀️ ${t('menuLight')}` : `🌙 ${t('menuDark')}` }}
        </button>
      </nav>
    </div>

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
        <p :key="tip" class="tip-fade hidden text-sm text-slate-400 sm:block">{{ t('tips')[tip] }}</p>

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
              class="inline-flex items-center gap-1.5 rounded-full border border-dashed border-amber-400/50 bg-amber-400/5 px-4 py-1 text-sm font-bold text-amber-300/80 hover:border-amber-400 hover:bg-amber-400/15 hover:text-amber-200 light:text-amber-700 light:hover:text-amber-800"
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

          <p class="text-center text-xs text-slate-500">
            {{ t('gainInfo').replace('{n}', String(displayGain))
            }}<template v-if="displayCoins"> · 💰+{{ displayCoins }}</template>
          </p>

          <p class="h-5 text-center text-sm font-bold sm:h-6 sm:text-base" :class="messageColor">{{ message }}</p>

          <button
            v-if="state.win"
            class="animate-pulse rounded-full border-2 border-fuchsia-400 bg-fuchsia-500/20 px-8 py-3 text-lg font-black tracking-widest text-fuchsia-200 hover:bg-fuchsia-500/30 light:border-fuchsia-600 light:text-fuchsia-700"
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
              <template v-if="risk">{{ t('chance') }} 1/{{ risk + 1 }}</template>
            </span>
          </div>

          <!-- wallet + the three shop entries share one wrapping row with quota and best -->
          <div class="flex flex-wrap items-center justify-center gap-x-4 gap-y-1.5">
            <div id="tut-shop" class="flex flex-wrap items-center justify-center gap-2">
              <span ref="coinEl" class="font-mono text-sm font-bold text-yellow-300 light:text-yellow-700">
                💰 {{ shownCoins }}
              </span>
              <button
                id="tut-shop-sell"
                class="rounded-full border border-yellow-500/40 bg-yellow-400/10 px-3 py-1 text-sm font-bold text-yellow-300 hover:bg-yellow-400/20 disabled:opacity-40 light:text-yellow-700"
                :disabled="!streakSellValue || state.win"
                :title="state.win ? t('sellAtWin') : t('sellDesc')"
                @click="sellAsk = true; play('switch')"
              >
                ⭐ {{ t('sellStreak') }} +{{ streakSellValue }}💰
              </button>
<!-- phones reach the shops through the hamburger; keeping the pills too
                   was one row of clutter more than the core loop needs -->
              <button
                class="hidden rounded-full border border-violet-500/40 bg-violet-500/10 px-3 py-1 text-sm font-bold text-violet-300 hover:bg-violet-500/20 light:text-violet-700 lg:block"
                @click="showShop = 'items'; play('switch')"
              >
                🎁 {{ t('itemShop') }}
              </button>
              <button
                class="hidden rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1 text-sm font-bold text-emerald-300 hover:bg-emerald-500/20 light:text-emerald-700 lg:block"
                @click="showShop = 'skills'; play('switch')"
              >
                📈 {{ t('skillShop') }}
              </button>
            </div>

            <p id="tut-quota" class="text-sm text-slate-400">
              <template v-if="state.quotaLeft > 0">
                {{ t('clicksLeft') }}:
                <span class="font-mono font-bold" :class="quotaColor">{{ state.quotaLeft }}</span>
                / {{ state.quota }}
              </template>
              <template v-else>
                {{ t('quotaExhausted') }}
                <span class="font-mono font-bold text-slate-200">⏳ {{ refillIn }}</span>
              </template>
            </p>

            <p class="text-xs text-slate-500">{{ t('best') }}: ★{{ state.bestStars }}</p>
          </div>

          <!-- phones: rankings/collection/shops all live behind the ☰ drawer;
               lg lays the panels out as grid columns instead -->
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
      @sell="onSellCard"
    />
    <NicknameModal
      v-if="ready && showNickname"
      @close="showNickname = false"
      @link="showNickname = false; linkViaNickname = true; showLink = true"
    />
    <LinkModal v-if="showLink" @close="closeLink" />
    <ShopModal v-if="showShop" :kind="showShop" @close="showShop = ''" />
    <ConfirmModal
      v-if="sellAsk && state"
      :title="`⭐ ${t('sellStreak')}`"
      :message="
        t('sellStreakConfirm')
          .replace('{n}', String(state.stars))
          .replace('{c}', String(streakSellValue))
      "
      :confirm-label="t('sellStreak')"
      :cancel-label="t('later')"
      @confirm="confirmSellStreak"
      @cancel="sellAsk = false"
    />
    <OddsModal v-if="showOdds" @close="showOdds = false" />
    <TalismanPicker v-if="showTalismanPick" @close="showTalismanPick = false" />
    <ConfirmModal
      v-if="sellCardAsk"
      :title="`💰 ${t('sellCard')}`"
      :message="t('sellCardConfirm')"
      :confirm-label="t('sellCard')"
      :cancel-label="t('later')"
      @confirm="confirmSellCard"
      @cancel="sellCardAsk = null"
    />
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
