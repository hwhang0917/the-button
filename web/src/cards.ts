import { t } from './i18n'
import type { Rarity, Tier } from './tiers'

/** Mirrors cardEffect in game.go. The server resolves every click — these
 *  fields exist so the UI can name a card and show what it will do. */
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

export type CardKey = `${Tier}/${Rarity}`

export const cardKey = (tier: Tier, rarity: Rarity) => `${tier}/${rarity}` as CardKey

/** Mirrors cardEffects in game.go — rarity is the power band, the six tier
 *  variants inside a band differ in flavour rather than strength. */
export const CARD_EFFECTS: Record<CardKey, CardEffect> = {
  // 커먼
  'unrank/common': { chance: 5 },
  'bronze/common': { refund: true },
  'silver/common': { bonus: 1 },
  'gold/common': { coinWin: 2 },
  'platinum/common': { half: true },
  'diamond/common': { coinLoss: 3 },

  // 레어
  'unrank/rare': { chance: 10 },
  'bronze/rare': { keep: true, refund: true },
  'silver/rare': { rerolls: 1 },
  'gold/rare': { chance: 5, bonus: 1 },
  'platinum/rare': { bonus: 2 },
  'diamond/rare': { card: true },

  // 홀로
  'unrank/holo': { chance: 20 },
  'bronze/holo': { keep: true },
  'silver/holo': { tierJump: true },
  'gold/holo': { mult: 2 },
  'platinum/holo': { rerolls: 2 },
  'diamond/holo': { maxRisk: true },

  // 프리즘
  'unrank/prismatic': { guarantee: true },
  'bronze/prismatic': { mult: 3 },
  'silver/prismatic': { guarantee: true, refund: true },
  'gold/prismatic': { guarantee: true, bonus: 2 },
  'platinum/prismatic': { guarantee: true, bestJump: true },
  'diamond/prismatic': { guarantee: true, bonus: 4, card: true },
}

export const effectFor = (tier: Tier, rarity: Rarity): CardEffect =>
  CARD_EFFECTS[cardKey(tier, rarity)] ?? {}

export const cardName = (tier: Tier, rarity: Rarity): string =>
  t('cardName')[cardKey(tier, rarity)] ?? ''

/** The roll bonus shown as a `+N%` tag on the button while a card is armed. */
export const cardChance = (tier: Tier, rarity: Rarity): number => effectFor(tier, rarity).chance ?? 0

/** Builds a card's description out of the same fields the server resolves, so
 *  the copy can never drift from what the card actually does. */
export function effectText(tier: Tier, rarity: Rarity): string {
  const e = effectFor(tier, rarity)
  const fx = t('cardFx')
  const n = (s: string, v: number) => s.replace('{n}', String(v))
  const parts: string[] = []
  if (e.chance) parts.push(n(fx.chance, e.chance))
  if (e.guarantee) parts.push(fx.guarantee)
  if (e.bonus) parts.push(n(fx.bonus, e.bonus))
  if (e.mult) parts.push(n(fx.mult, e.mult))
  if (e.maxRisk) parts.push(fx.maxRisk)
  if (e.tierJump) parts.push(fx.tierJump)
  if (e.bestJump) parts.push(fx.bestJump)
  if (e.keep) parts.push(fx.keep)
  if (e.half) parts.push(fx.half)
  if (e.rerolls) parts.push(n(fx.rerolls, e.rerolls))
  if (e.refund) parts.push(fx.refund)
  if (e.coinWin) parts.push(n(fx.coinWin, e.coinWin))
  if (e.coinLoss) parts.push(n(fx.coinLoss, e.coinLoss))
  if (e.card) parts.push(fx.card)
  return parts.join(' · ')
}
