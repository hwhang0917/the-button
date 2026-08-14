import { ref } from 'vue'
import { cardChance } from './cards'
import { cfg } from './config'
import { RARITIES, type Rarity, type Tier } from './tiers'

export interface GameState {
  stars: number
  bestStars: number
  tier: Tier
  chance: number
  maxStars: number
  quotaLeft: number
  quota: number
  nickname: string
  win: boolean
  coins: number
  charmLevel: number
  headstartLevel: number
  staminaLevel: number
  prestige: number
  talismanTier: Tier | ''
  talismanRarity: Rarity | ''
  refillUsed: boolean
  devMode: boolean
}

export interface Card {
  tier: Tier
  rarity: Rarity
}

export interface ClickResult {
  success: boolean
  stars: number
  gained: number
  tier: Tier
  tierUp: boolean
  win: boolean
  card: Card | null
  chance: number
  quotaLeft: number
  bonusClicks: number
  talismanUsed: boolean
  talismanTier: Tier | ''
  talismanRarity: Rarity | ''
  refund: boolean
  jackpot: number
  coins: number
}

export interface OwnedCard extends Card {
  count: number
}

export interface RankEntry {
  nickname: string
  stars: number
  bestStars: number
  tier: Tier
  prestige: number
}

export const state = ref<GameState | null>(null)
export const leaderboard = ref<RankEntry[]>([])
export const cards = ref<OwnedCard[]>([])

/* liveness probe: HEAD on an existing endpoint, no dedicated health route */
const HEALTH_INTERVAL_MS = 5000
export const offline = ref(false)

async function checkHealth() {
  if (document.hidden) return // backgrounded browsers cancel fetches; that's not an outage
  const wasOffline = offline.value
  try {
    offline.value = !(await fetch('/api/state', { method: 'HEAD' })).ok
  } catch {
    // a probe in flight when the tab hides gets killed mid-request — same false alarm
    if (!document.hidden) offline.value = true
  }
  if (wasOffline && !offline.value) loadState() // resync after an outage
}

export function startHealthCheck() {
  setInterval(checkHealth, HEALTH_INTERVAL_MS)
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) checkHealth()
  })
}

export async function loadState() {
  state.value = await (await fetch('/api/state')).json()
}

export async function loadLeaderboard() {
  leaderboard.value = await (await fetch('/api/leaderboard')).json()
}

export async function loadCards() {
  cards.value = await (await fetch('/api/cards')).json()
}

/** Stars won on success — round(100/chance), matching GainFor in game/resolve.go. */
export function gainFor(chance: number, risk: number): number {
  if (risk <= 0 || chance <= 0) return 1
  return Math.max(1, Math.round(100 / chance))
}

/** Roll chance after risk division and charm bonus. Mirrors EffChanceFor. */
export function effChance(base: number, risk: number, charm: number): number {
  return Math.min(100, Math.floor(base / (risk + 1)) + cfg().charm.bonusPct * charm)
}

const tri = (n: number) => (n * (n + 1)) / 2

/** Coin payout for selling the streak down to the head-start floor. Mirrors StreakValue. */
export function streakValue(stars: number, floor: number): number {
  return tri(stars) - tri(Math.min(floor, stars))
}

export type SkillKey = 'charm' | 'headstart' | 'stamina'

/** A skill's ladder; the cap is however many prices the server published. */
export function skill(key: SkillKey): { prices: number[]; cap: number } {
  const s = cfg()[key]
  return { prices: s.prices, cap: s.prices.length }
}

/** Payout for prestiging from the given level; laps past the ladder pay its top. */
export function prestigeReward(prestige: number): number {
  const rewards = cfg().prestigeRewards
  return rewards[Math.min(prestige, rewards.length - 1)]
}

/** Converts a maxed streak to points + a star-tier promotion. Returns points gained. */
export async function prestigeStreak(): Promise<number | null> {
  const res = await fetch('/api/prestige', { method: 'POST' })
  if (!res.ok) return null
  const d = await res.json()
  if (state.value) {
    Object.assign(state.value, {
      coins: d.coins,
      stars: d.stars,
      tier: d.tier,
      chance: d.chance,
      prestige: d.prestige,
      win: false,
    })
  }
  loadLeaderboard() // prestige level and stars both feed the ranking
  return d.gained
}

/** The armed card's chance bonus — it fires on the next click whatever the tier. */
export function talismanBonus(s: GameState): number {
  if (!s.talismanTier || !s.talismanRarity) return 0
  return cardChance(s.talismanTier, s.talismanRarity)
}

/** Consumes one copy of a card and arms it as the single talisman slot. */
export async function armTalisman(tier: Tier, rarity: Rarity): Promise<boolean> {
  const res = await fetch('/api/talisman', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tier, rarity }),
  })
  if (!res.ok) return false
  const d = await res.json()
  if (state.value) {
    state.value.talismanTier = d.talismanTier
    state.value.talismanRarity = d.talismanRarity
  }
  await loadCards()
  return true
}

/** Disarms the talisman slot and refunds the card copy. */
export async function cancelTalisman(): Promise<boolean> {
  const res = await fetch('/api/talisman/cancel', { method: 'POST' })
  if (!res.ok) return false
  if (state.value) {
    state.value.talismanTier = ''
    state.value.talismanRarity = ''
  }
  await loadCards()
  return true
}

/** Rarity one step down; null for the bottom of the ladder. */
export function prevRarity(r: Rarity): Rarity | null {
  const i = RARITIES.indexOf(r)
  return i > 0 ? RARITIES[i - 1] : null
}

/** Breaks one card into cfg().defuseYield copies of the rarity below. */
export async function defuseCard(tier: Tier, rarity: Rarity): Promise<boolean> {
  const res = await fetch('/api/defuse', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tier, rarity }),
  })
  if (res.ok) await loadCards()
  return res.ok
}

/** Burns cfg().fuseCost copies of a card into 1 of the next rarity, same tier. */
export async function fuseCards(tier: Tier, rarity: Rarity): Promise<boolean> {
  const res = await fetch('/api/fuse', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tier, rarity }),
  })
  if (res.ok) await loadCards()
  return res.ok
}

/** Fusion target one rarity up; null at the top of the ladder. */
export function nextRarity(r: Rarity): Rarity | null {
  const i = RARITIES.indexOf(r)
  return i >= 0 && i + 1 < RARITIES.length ? RARITIES[i + 1] : null
}

/** Buys one card pack; returns its 1-3 cards (coins patched immediately). */
export async function buyPack(): Promise<Card[] | null> {
  const res = await fetch('/api/pack', { method: 'POST' })
  if (!res.ok) return null
  const d = await res.json()
  if (state.value) state.value.coins = d.coins
  return d.cards
}

/** Buys back this hour's spent clicks (once per day). */
export async function refillQuota(): Promise<boolean> {
  const res = await fetch('/api/refill', { method: 'POST' })
  if (!res.ok) return false
  const d = await res.json()
  if (state.value) {
    state.value.coins = d.coins
    state.value.quotaLeft = d.quotaLeft
    state.value.refillUsed = true
  }
  return true
}

/** Buys a scratch ticket. Patches state with the prize still hidden (price
 * deducted only) — the LotteryModal applies `coins` after the reveal. */
export async function buyLottery(): Promise<{ prize: number; coins: number } | null> {
  const res = await fetch('/api/lottery', { method: 'POST' })
  if (!res.ok) return null
  const d = await res.json()
  if (state.value) state.value.coins = d.coins - d.prize
  return d
}

/** Sells the whole streak; returns coins gained, or null when rejected. */
export async function sellStreak(): Promise<number | null> {
  const res = await fetch('/api/sell', { method: 'POST' })
  if (!res.ok) return null
  const d = await res.json()
  if (state.value) {
    Object.assign(state.value, { coins: d.coins, stars: d.stars, tier: d.tier, chance: d.chance, win: false })
  }
  loadLeaderboard() // rank sorts by current stars, so a sell moves the board
  return d.gained
}

export async function buySkill(skill: SkillKey): Promise<boolean> {
  const res = await fetch('/api/buy', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ skill }),
  })
  if (!res.ok) return false
  const d = await res.json()
  if (state.value) {
    Object.assign(state.value, {
      coins: d.coins,
      charmLevel: d.charmLevel,
      headstartLevel: d.headstartLevel,
      staminaLevel: d.staminaLevel,
      quota: d.quota, // stamina raises the hourly allowance
    })
  }
  return true
}

/** Returns the roll result, or null when out of quota / already won. */
export async function click(risk: number): Promise<ClickResult | null> {
  const res = await fetch('/api/click', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ risk }),
  })
  if (!res.ok) {
    await loadState()
    return null
  }
  const result: ClickResult = await res.json()
  if (state.value) {
    state.value.stars = result.stars
    state.value.tier = result.tier
    state.value.chance = result.chance
    state.value.quotaLeft = result.quotaLeft
    state.value.win = result.win
    state.value.talismanTier = result.talismanTier
    state.value.talismanRarity = result.talismanRarity
    state.value.coins = result.coins
    if (result.stars > state.value.bestStars) state.value.bestStars = result.stars
  }
  return result
}

export async function saveNickname(name: string): Promise<'ok' | 'taken' | 'error'> {
  const res = await fetch('/api/nickname', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (res.ok) {
    if (state.value) state.value.nickname = name
    return 'ok'
  }
  return res.status === 409 ? 'taken' : 'error'
}

/** Wipes rank, cards, and quota server-side; the session cookie is cleared. */
export async function deletePlayer(): Promise<boolean> {
  const res = await fetch('/api/player', { method: 'DELETE' })
  return res.ok
}

/** Mints a one-time code (valid 10 min) another device can claim to log into this account. */
export async function newLinkCode(): Promise<string | null> {
  const res = await fetch('/api/link/new', { method: 'POST' })
  if (!res.ok) return null
  return (await res.json()).code
}

/** Swaps this device's session for the account behind the code. Caller reloads on true. */
export async function claimLinkCode(code: string): Promise<boolean> {
  const res = await fetch('/api/link/claim', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  })
  return res.ok
}
