package instrument

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

// Registry 进程内合约元数据索引（Connect 后从 InstrumentSource 加载）。
type Registry struct {
	mu         sync.RWMutex
	byContract map[perp.Contract]*perp.ContractMeta
	byPair     map[spot.Pair]*spot.PairMeta
}

var _ ContractMetaLookup = (*Registry)(nil)

func NewRegistry() *Registry {
	return &Registry{
		byContract: make(map[perp.Contract]*perp.ContractMeta),
		byPair:     make(map[spot.Pair]*spot.PairMeta),
	}
}

var _ ContractMetaLookup = (*Registry)(nil)

// LoadFrom 从 InstrumentSource 拉取并重建索引；失败时不修改已有索引。
func (r *Registry) LoadFrom(ctx context.Context, src perp.InstrumentSource) error {
	if r == nil {
		return fmt.Errorf("instrument: nil registry")
	}
	list, err := src.ListContractMeta(ctx)
	if err != nil {
		return err
	}
	m := make(map[perp.Contract]*perp.ContractMeta, len(list))
	for _, meta := range list {
		if meta == nil || meta.Contract == "" {
			continue
		}
		cp := *meta
		m[meta.Contract] = &cp
	}
	r.mu.Lock()
	r.byContract = m
	r.mu.Unlock()
	return nil
}

// ContractMeta 返回合约元数据副本；未命中返回 error。
func (r *Registry) ContractMeta(contract perp.Contract) (*perp.ContractMeta, error) {
	if r == nil {
		return nil, fmt.Errorf("contract meta not found for %s", contract)
	}
	r.mu.RLock()
	meta, ok := r.byContract[contract]
	r.mu.RUnlock()
	if !ok || meta == nil {
		return nil, fmt.Errorf("contract meta not found for %s", contract)
	}
	cp := *meta
	return &cp, nil
}

// Len 返回已索引合约数。
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	n := len(r.byContract)
	r.mu.RUnlock()
	return n
}

func (r *Registry) LoadPairsFrom(ctx context.Context, src spot.InstrumentSource) error {
	if r == nil {
		return fmt.Errorf("instrument: nil registry")
	}
	list, err := src.ListPairMeta(ctx)
	if err != nil {
		return err
	}
	m := make(map[spot.Pair]*spot.PairMeta, len(list))
	for _, meta := range list {
		if meta == nil || meta.Pair == "" {
			continue
		}
		cp := *meta
		m[meta.Pair] = &cp
	}
	r.mu.Lock()
	r.byPair = m
	r.mu.Unlock()
	return nil
}

func (r *Registry) PairMeta(pair spot.Pair) (*spot.PairMeta, error) {
	if r == nil {
		return nil, fmt.Errorf("pair meta not found for %s", pair)
	}
	r.mu.RLock()
	meta, ok := r.byPair[pair]
	r.mu.RUnlock()
	if !ok || meta == nil {
		return nil, fmt.Errorf("pair meta not found for %s", pair)
	}
	cp := *meta
	return &cp, nil
}

func (r *Registry) AllPairs() []spot.Pair {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pairs := make([]spot.Pair, 0, len(r.byPair))
	for p := range r.byPair {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		return string(pairs[i]) < string(pairs[j])
	})
	return pairs
}
