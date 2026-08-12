package game

import (
	"crypto/rand"
	"math/big"
)

// RNG is every source of chance in the game. It is an interface so DEV_MODE is
// a swapped implementation rather than a global flag inside the roll, and so
// tests can script exact outcomes instead of looping until a rare roll lands.
type RNG interface {
	// Pct reports whether a pct% roll succeeds.
	Pct(pct int) bool
	// Intn returns a value in [0, n). n is always positive.
	Intn(n int) int
}

// CryptoRNG draws from crypto/rand so results cannot be predicted or replayed
// by clients.
type CryptoRNG struct{}

func (CryptoRNG) Pct(pct int) bool { return CryptoRNG{}.Intn(100) < pct }

func (CryptoRNG) Intn(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// crypto/rand failing means the OS entropy source is broken; nothing
		// sensible to do but return the losing end of the range.
		return n - 1
	}
	return int(v.Int64())
}

// AlwaysWin backs DEV_MODE: every percentage roll succeeds. Intn still varies
// so pack contents stay interesting while clicks are rigged.
type AlwaysWin struct{ CryptoRNG }

func (AlwaysWin) Pct(int) bool { return true }
