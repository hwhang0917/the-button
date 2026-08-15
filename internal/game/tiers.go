package game

// Tier is one rung of the star ladder: everything from MinStars up to the next
// tier's MinStars carries this name.
type Tier struct {
	Name     string `yaml:"name"      json:"name"`
	MinStars int    `yaml:"min_stars" json:"minStars"`
}

// DefaultTiers is the shipped ladder. Validate requires ascending MinStars
// starting at 0, so tierFor always finds a match.
func DefaultTiers() []Tier {
	return []Tier{
		{"unrank", 0},
		{"bronze", 1},
		{"silver", 4},
		{"gold", 7},
		{"platinum", 10},
		{"diamond", 13},
	}
}

// DefaultRarities is the shipped card-rarity ladder, weakest first. Fusion
// walks up it and defusion walks down.
func DefaultRarities() []string { return []string{"common", "rare", "holo", "prismatic"} }

// scaledMin spreads a tier threshold in ratio: MinStars is defined against the
// base MaxStars, so a prestige-raised cap stretches the whole ladder with it.
// cap >= MaxStars keeps the scaled thresholds strictly ascending.
func (r Rules) scaledMin(minStars, cap int) int {
	return minStars * cap / r.MaxStars
}

// TierFor is the tier holding the given star count at the given cap.
func (r Rules) TierFor(stars, cap int) string {
	name := r.Tiers[0].Name
	for _, t := range r.Tiers {
		if stars >= r.scaledMin(t.MinStars, cap) {
			name = t.Name
		}
	}
	return name
}

// TierRank is a tier's position on the ladder, 0 for the lowest. Doubles as the
// bonus-click reward for reaching it.
func (r Rules) TierRank(name string) int {
	for i, t := range r.Tiers {
		if t.Name == name {
			return i
		}
	}
	return 0
}

// NextTierMin is the first star count of the tier above the one holding stars,
// or 0 at the top of the ladder — the 🌙 달빛 사다리 card jumps to it.
func (r Rules) NextTierMin(stars, cap int) int {
	for _, t := range r.Tiers {
		if m := r.scaledMin(t.MinStars, cap); m > stars {
			return m
		}
	}
	return 0
}

func (r Rules) ValidTier(name string) bool {
	for _, t := range r.Tiers {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (r Rules) ValidRarity(name string) bool {
	for _, x := range r.Rarities {
		if x == name {
			return true
		}
	}
	return false
}

// NextRarity is the fusion target one step up the ladder; the top is final.
func (r Rules) NextRarity(rarity string) (string, bool) {
	for i, name := range r.Rarities {
		if name == rarity && i+1 < len(r.Rarities) {
			return r.Rarities[i+1], true
		}
	}
	return "", false
}

// PrevRarity is the defusion target one step down; the bottom is final.
func (r Rules) PrevRarity(rarity string) (string, bool) {
	for i, name := range r.Rarities {
		if name == rarity && i > 0 {
			return r.Rarities[i-1], true
		}
	}
	return "", false
}
