package strategies

import (
	"hash/fnv"
	"sync"
	"time"
)

type PendingBuyRepo interface {
	Add(pb *PendingBuy)
	Confirm(pair string, ok func(pb *PendingBuy) bool) (confirmed *PendingBuy)
	Exists(pair string) bool
}

const shardCount = 32 // power of two keeps masking cheap

type shard struct {
	sync.RWMutex
	m map[string]*PendingBuy
}

type shardedRepo struct {
	s [shardCount]shard
}

func NewPendingBuyRepo() PendingBuyRepo {
	r := &shardedRepo{}
	for i := range r.s {
		r.s[i].m = make(map[string]*PendingBuy, 4)
	}
	return r
}

func (r *shardedRepo) Add(pb *PendingBuy) {
	sh := r.shard(pb.Pair)
	sh.Lock()
	sh.m[pb.ID] = pb
	sh.Unlock()
}

func (r *shardedRepo) Exists(pair string) bool {
	sh := r.shard(pair)
	sh.RLock()
	defer sh.RUnlock()
	for _, pb := range sh.m {
		if pb.Pair == pair {
			return true
		}
	}
	return false
}

func (r *shardedRepo) Confirm(pair string, ok func(pb *PendingBuy) bool) *PendingBuy {
	sh := r.shard(pair)

	sh.Lock()
	defer sh.Unlock()

	for id, pb := range sh.m {
		if pb.Pair != pair {
			continue
		}
		if ok(pb) {
			delete(sh.m, id) // consume
			return pb
		}
		// kill stale entries (> 24 h) to avoid leaks
		if time.Since(pb.TriggerTime) > 24*time.Hour {
			delete(sh.m, id)
		}
	}
	return nil
}

func (r *shardedRepo) shard(pair string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(pair))
	return &r.s[h.Sum32()&(shardCount-1)]
}
