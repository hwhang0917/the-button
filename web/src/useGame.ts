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
}

export interface OwnedCard extends Card {
  count: number
}

export interface RankEntry {
  nickname: string
  stars: number
  bestStars: number
  tier: Tier
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
