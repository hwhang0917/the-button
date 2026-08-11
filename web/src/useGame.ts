import { ref } from 'vue'
import type { Rarity, Tier } from './tiers'

export interface GameState {
  stars: number
  bestStars: number
  tier: Tier
  chance: number
  quotaLeft: number
  quota: number
  nickname: string
  win: boolean
  coins: number
  shieldCharges: number
  charmLevel: number
  headstartLevel: number
  prestige: number
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
  shieldUsed: boolean
  shieldCharges: number
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
  earned: number
}

export const state = ref<GameState | null>(null)
export const leaderboard = ref<RankEntry[]>([])
export const cards = ref<OwnedCard[]>([])

export async function loadState() {
  state.value = await (await fetch('/api/state')).json()
}

export async function loadLeaderboard() {
  leaderboard.value = await (await fetch('/api/leaderboard')).json()
}

export async function loadCards() {
  cards.value = await (await fetch('/api/cards')).json()
}

/** Risk levels 0 (safe) to 3: odds ÷(level+1); success pays the odds back. Mirrors game.go. */
export const MAX_RISK = 3

/** Stars won on success — round(100/chance), matching gainFor in game.go. */
export function gainFor(chance: number, risk: number): number {
  if (risk <= 0 || chance <= 0) return 1
  return Math.max(1, Math.round(100 / chance))
}

/** Roll chance after risk division and charm bonus. Mirrors effChanceFor in game.go. */
export function effChance(base: number, risk: number, charm: number): number {
  return Math.min(100, Math.floor(base / (risk + 1)) + 2 * charm)
}

const tri = (n: number) => (n * (n + 1)) / 2

/** Coin payout for selling the streak down to the head-start floor. Mirrors streakValue in game.go. */
export function streakValue(stars: number, floor: number): number {
  return tri(stars) - tri(Math.min(floor, stars))
}

export type SkillKey = 'shield' | 'charm' | 'headstart'

/** Mirrors the price ladders and caps in game.go. */
export const SKILLS: Record<SkillKey, { prices: number[]; cap: number }> = {
  shield: { prices: [25], cap: Infinity }, // flat price, uncapped charges
  charm: { prices: [10, 30, 90, 270, 810], cap: 5 },
  headstart: { prices: [20, 100, 400], cap: 3 },
}

/** Mirrors prestigeRewards in game.go; index = current prestige (capped). */
export const PRESTIGE_REWARDS = [300, 450, 600]
export const PRESTIGE_CAP = 3

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
  return d.gained
}

/** Mirrors lotteryPrice in game.go. */
export const LOTTERY_PRICE = 15

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
      shieldCharges: d.shieldCharges,
      charmLevel: d.charmLevel,
      headstartLevel: d.headstartLevel,
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
    state.value.shieldCharges = result.shieldCharges
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
