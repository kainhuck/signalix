package spot

import "github.com/kainhuck/signalix/pkg/exchange/spot"

type pairCandleKey struct {
	Pair     spot.Pair
	Interval string
}
