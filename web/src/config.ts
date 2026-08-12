import { ref } from 'vue'
import type { Rarity, Tier } from './tiers'

/** One card's effect. Shape mirrors game.CardEffect; absent fields are off. */
export interface CardEffect {
  chance?: number
  guarantee?: boolean
  bonus?: number
  mult?: number
  maxRisk?: boolean
  tierJump?: boolean
  bestJump?: boolean
  keep?: boolean
  half?: boolean
  rerolls?: number
  refund?: boolean
  coinWin?: number
  coinLoss?: number
  card?: boolean
}

export interface Weight {
  name: string
  permille: number
}

/** Everything the server lets config.yml retune that the UI has to render.
 *  Fetched once at boot so prices, odds and card effects live in exactly one
 *  place — hardcoding them here would go stale the moment config.yml changed. */
export interface ServerConfig {
  chanceTable: number[]
  maxRisk: number
  maxStars: number
  prestigeStarBonus: number
  prestigeSkinCap: number
  prestigeRewards: number[]
  overflowCoinPer: number
  tiers: { name: Tier; minStars: number }[]
  rarities: Rarity[]
  cards: Record<string, CardEffect>
  charm: { bonusPct: number; prices: number[] }
  headstart: { bonusPct: number; prices: number[] }
  lottery: { price: number; prizes: { prize: number; permille: number }[] }
  pack: { price: number; bonusPct: number[]; tiers: Weight[]; rarities: Weight[] }
  refillPrice: number
  fuseCost: number
  defuseYield: number
  nickname: { minLen: number; maxLen: number }
}

const loaded = ref<ServerConfig | null>(null)

export async function loadConfig() {
  loaded.value = await (await fetch('/api/config')).json()
}

/** The server config. Everything that reads it renders behind App's `ready`
 *  gate, so the throw only ever fires if that ordering is broken. */
export function cfg(): ServerConfig {
  if (!loaded.value) throw new Error('config read before /api/config finished loading')
  return loaded.value
}
