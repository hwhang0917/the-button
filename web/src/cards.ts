import { cfg, type CardEffect } from './config'
import { t } from './i18n'
import type { Rarity, Tier } from './tiers'

export type { CardEffect }

export type CardKey = `${Tier}/${Rarity}`

export const cardKey = (tier: Tier, rarity: Rarity) => `${tier}/${rarity}` as CardKey

/** The card's effect, as published by the server — config.yml owns these. */
export const effectFor = (tier: Tier, rarity: Rarity): CardEffect =>
  cfg().cards[cardKey(tier, rarity)] ?? {}

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
