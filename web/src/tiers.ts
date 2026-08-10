export const TIERS = ['unrank', 'bronze', 'silver', 'gold', 'platinum', 'diamond'] as const
export type Tier = (typeof TIERS)[number]

export const TIER_COLORS: Record<Tier, string> = {
  unrank: '#9ca3af',
  bronze: '#cd7f32',
  silver: '#d8dee9',
  gold: '#facc15',
  platinum: '#67e8f9',
  diamond: '#a78bfa',
}

export const RARITIES = ['common', 'rare', 'holo', 'prismatic'] as const
export type Rarity = (typeof RARITIES)[number]
