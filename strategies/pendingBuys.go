package strategies

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
)

type PendingBuy struct {
	ID           string             `json:"id"`
	Pair         string             `json:"pair"`
	TriggerPrice float64            `json:"trigger_price"`
	TriggerTime  time.Time          `json:"trigger_time"`
	RsiVal       float64            `json:"rsi_val"`
	MacdLine     float64            `json:"macd_line"`
	MacdSignal   float64            `json:"macd_signal"`
	MarketState  models.MarketState `json:"market_state"`

	Volume         float64 `json:"volume"`
	ATR            float64 `json:"atr"`
	BollingerWidth float64 `json:"bollinger_width"`
	StochasticK    float64 `json:"stochastic_k"`
	StochasticD    float64 `json:"stochastic_d"`
	CCIValue       float64 `json:"cci_value"`
	MFIValue       float64 `json:"mfi_value"`

	TrendStrength float64 `json:"trend_strength"`
	PricePosition float64 `json:"price_position"`
	VolumeRatio   float64 `json:"volume_ratio"`

	ConfidenceScore float64 `json:"confidence_score"`
	Priority        int     `json:"priority"`
	CreatedBy       string  `json:"created_by"`

	lastAccessed time.Time
	accessCount  int64

	lastOkAt time.Time
	okCount  int
}

type PendingBuyRepo interface {
	Add(pb *PendingBuy) error
	AddOrReplace(pb *PendingBuy) (*PendingBuy, error)
	Confirm(pair string, ok func(pb *PendingBuy) bool) *PendingBuy
	Exists(pair string) bool
	ExistsWithCondition(pair string, condition func(pb *PendingBuy) bool) bool
	Get(pair string) *PendingBuy
	GetAll(pair string) []*PendingBuy
	GetBest(pair string) *PendingBuy
	Remove(pair string, id string) bool
	RemoveAll(pair string) int
	Count() int
	CountByPair(pair string) int
	Cleanup() int // removes stale entries, returns count removed
	GetStats() *RepoStats
	Close() error
}

type RepoStats struct {
	TotalCount    int            `json:"total_count"`
	PairCounts    map[string]int `json:"pair_counts"`
	AvgAgeMinutes float64        `json:"avg_age_minutes"`
	OldestEntry   time.Time      `json:"oldest_entry"`
	NewestEntry   time.Time      `json:"newest_entry"`
	AvgConfidence float64        `json:"avg_confidence"`
	MarketStates  map[string]int `json:"market_states"`
	PriorityDist  map[int]int    `json:"priority_distribution"`
}

const (
	shardCount      = 64
	maxAgeHours     = 6  // slightly shorter living; confirmation rules are looser now
	cleanupInterval = 15 // minutes
	maxPairEntries  = 10

	confirmMinAge = 3 * time.Minute

	minObservations = 2

	minEvalSpacing = 30 * time.Second
)

type shard struct {
	sync.RWMutex
	m           map[string]*PendingBuy
	lastCleanup time.Time
	accessCount int64
}

type shardedRepo struct {
	shards        [shardCount]shard
	totalCount    int64
	cleanupTicker *time.Ticker
	stopCh        chan struct{}
	closed        int32
	mu            sync.RWMutex
}

func NewPendingBuyRepo() PendingBuyRepo {
	r := &shardedRepo{
		cleanupTicker: time.NewTicker(cleanupInterval * time.Minute),
		stopCh:        make(chan struct{}),
	}

	for i := range r.shards {
		r.shards[i].m = make(map[string]*PendingBuy, maxPairEntries*4)
		r.shards[i].lastCleanup = time.Now()
	}

	go r.backgroundCleanup()

	return r
}

func (r *shardedRepo) Add(pb *PendingBuy) error {
	if pb == nil {
		return fmt.Errorf("pending buy cannot be nil")
	}
	if atomic.LoadInt32(&r.closed) == 1 {
		return fmt.Errorf("repository is closed")
	}
	if pb.ID == "" {
		pb.ID = r.generateID()
	}

	pb.lastAccessed = time.Now()
	pb.accessCount = 1
	pb.lastOkAt = time.Time{}
	pb.okCount = 0

	sh := r.getShard(pb.Pair)
	sh.Lock()
	defer sh.Unlock()

	pairCount := r.countPairInShard(sh, pb.Pair)
	if pairCount >= maxPairEntries {
		r.removeWorstInShard(sh, pb.Pair)
	}

	sh.m[pb.ID] = pb
	atomic.AddInt64(&r.totalCount, 1)
	atomic.AddInt64(&sh.accessCount, 1)

	logger.Infof("[PENDING_BUY] Added %s for %s (ID: %s, Score: %.3f)",
		pb.CreatedBy, pb.Pair, pb.ID[:8], r.calculateScore(pb, time.Now()))

	return nil
}

func (r *shardedRepo) AddOrReplace(pb *PendingBuy) (*PendingBuy, error) {
	if pb == nil {
		return nil, fmt.Errorf("pending buy cannot be nil")
	}
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil, fmt.Errorf("repository is closed")
	}
	if pb.ID == "" {
		pb.ID = r.generateID()
	}

	pb.lastAccessed = time.Now()
	pb.accessCount = 1
	pb.lastOkAt = time.Time{}
	pb.okCount = 0

	sh := r.getShard(pb.Pair)
	sh.Lock()
	defer sh.Unlock()

	pairCount := r.countPairInShard(sh, pb.Pair)
	if pairCount < maxPairEntries {
		sh.m[pb.ID] = pb
		atomic.AddInt64(&r.totalCount, 1)
		atomic.AddInt64(&sh.accessCount, 1)
		logger.Infof("[PENDING_BUY] Added %s for %s (ID: %s, Score: %.3f)",
			pb.CreatedBy, pb.Pair, pb.ID[:8], r.calculateScore(pb, time.Now()))
		return nil, nil
	}

	var replaced *PendingBuy
	var replacedID string
	worstScore := math.Inf(1)
	now := time.Now()

	for id, existing := range sh.m {
		if existing.Pair != pb.Pair {
			continue
		}
		score := r.calculateScore(existing, now)
		if score < worstScore {
			worstScore = score
			replaced = existing
			replacedID = id
		}
	}

	newScore := r.calculateScore(pb, now)

	shouldReplace := newScore > worstScore*1.04
	if !shouldReplace && replaced != nil {
		ageStale := now.Sub(replaced.TriggerTime) > 35*time.Minute
		priceDiff := 0.0
		if replaced.TriggerPrice > 0 {
			priceDiff = (replaced.TriggerPrice - pb.TriggerPrice) / replaced.TriggerPrice
		}
		shouldReplace = ageStale || (priceDiff > 0.012)
	}

	if shouldReplace {
		delete(sh.m, replacedID)
		logger.InfoColorf(logger.BrightGreen,
			"[PENDING BUY] Replaced for %s (old score: %.3f, new score: %.3f)",
			pb.Pair, worstScore, newScore)
		sh.m[pb.ID] = pb
		atomic.AddInt64(&sh.accessCount, 1)
		return replaced, nil
	}

	logger.Debugf("[PENDING_BUY] Rejected new candidate for %s (new score: %.3f vs worst: %.3f)",
		pb.Pair, newScore, worstScore)
	return nil, nil
}

func (r *shardedRepo) Confirm(pair string, ok func(pb *PendingBuy) bool) *PendingBuy {
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil
	}

	sh := r.getShard(pair)
	sh.Lock()
	defer sh.Unlock()

	now := time.Now()

	type candidate struct {
		id    string
		pb    *PendingBuy
		score float64
	}
	var cands []candidate

	for id, pb := range sh.m {
		if pb.Pair != pair {
			continue
		}

		age := now.Sub(pb.TriggerTime)

		// purge hard-stale entries
		if age > time.Duration(maxAgeHours)*time.Hour {
			delete(sh.m, id)
			atomic.AddInt64(&r.totalCount, -1)
			continue
		}

		// adaptive min age per state (but never less than base)
		minAge := r.adaptiveMinAge(pb.MarketState)
		if minAge < confirmMinAge {
			minAge = confirmMinAge
		}
		if age < minAge {
			continue
		}

		// Evaluate external predicate (bullish re-check, price sanity, etc.)
		if !ok(pb) {
			continue
		}

		// Only record a positive observation if spaced enough since last positive one
		needSpacing := r.adaptiveEvalSpacing(pb.MarketState)
		if pb.lastOkAt.IsZero() || now.Sub(pb.lastOkAt) >= needSpacing {
			pb.lastOkAt = now
			pb.okCount++
		}

		// Required observations
		reqObs := r.adaptiveMinObservations(pb.MarketState)
		if reqObs < minObservations {
			reqObs = minObservations
		}

		// Age relaxations: every 10 min reduce requirement by 1, floor to 1
		if age > 10*time.Minute {
			reqObs = max(1, reqObs-1)
		}
		if age > 25*time.Minute {
			reqObs = max(1, reqObs-1)
		}

		if pb.MarketState == models.RangeBound && pb.PricePosition <= 0.25 {
			reqObs = max(1, reqObs-1)
		}

		if (pb.MarketState == models.StronglyTrending && pb.okCount >= 1) ||
			(pb.MarketState == models.Trending && pb.okCount >= 2) {
			reqObs = 1
		}

		if pb.okCount < reqObs {
			continue
		}

		cands = append(cands, candidate{
			id:    id,
			pb:    pb,
			score: r.calculateScore(pb, now),
		})
	}

	if len(cands) == 0 {
		return nil
	}

	// Pick the highest score
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	best := cands[0]

	delete(sh.m, best.id)
	atomic.AddInt64(&r.totalCount, -1)

	logger.Infof("[PENDING BUY] Confirmed %s for %s (ID: %s, Age: %.1fm, PosObs:%d, Score: %.3f)",
		best.pb.CreatedBy, pair, best.pb.ID[:8], now.Sub(best.pb.TriggerTime).Minutes(), best.pb.okCount, best.score)

	return best.pb
}

func (r *shardedRepo) Exists(pair string) bool {
	if atomic.LoadInt32(&r.closed) == 1 {
		return false
	}

	sh := r.getShard(pair)
	sh.RLock()
	defer sh.RUnlock()

	now := time.Now()
	for _, pb := range sh.m {
		if pb.Pair == pair && now.Sub(pb.TriggerTime) <= time.Duration(maxAgeHours)*time.Hour {
			return true
		}
	}
	return false
}

func (r *shardedRepo) ExistsWithCondition(pair string, condition func(pb *PendingBuy) bool) bool {
	if atomic.LoadInt32(&r.closed) == 1 {
		return false
	}

	sh := r.getShard(pair)
	sh.RLock()
	defer sh.RUnlock()

	now := time.Now()
	for _, pb := range sh.m {
		if pb.Pair == pair &&
			now.Sub(pb.TriggerTime) <= time.Duration(maxAgeHours)*time.Hour &&
			condition(pb) {
			return true
		}
	}
	return false
}

func (r *shardedRepo) Get(pair string) *PendingBuy {
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil
	}

	sh := r.getShard(pair)
	sh.RLock()
	defer sh.RUnlock()

	var best *PendingBuy
	bestScore := math.Inf(-1)
	now := time.Now()

	for _, pb := range sh.m {
		if pb.Pair == pair && now.Sub(pb.TriggerTime) <= time.Duration(maxAgeHours)*time.Hour {
			score := r.calculateScore(pb, now)
			if score > bestScore {
				best = pb
				bestScore = score
			}
		}
	}

	if best != nil {
		atomic.AddInt64(&best.accessCount, 1)
		best.lastAccessed = now
	}

	return best
}

func (r *shardedRepo) GetAll(pair string) []*PendingBuy {
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil
	}

	sh := r.getShard(pair)
	sh.RLock()
	defer sh.RUnlock()

	var result []*PendingBuy
	now := time.Now()

	for _, pb := range sh.m {
		if pb.Pair == pair && now.Sub(pb.TriggerTime) <= time.Duration(maxAgeHours)*time.Hour {
			result = append(result, pb)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		scoreI := r.calculateScore(result[i], now)
		scoreJ := r.calculateScore(result[j], now)
		return scoreI > scoreJ
	})

	return result
}

func (r *shardedRepo) GetBest(pair string) *PendingBuy {
	return r.Get(pair)
}

func (r *shardedRepo) Remove(pair string, id string) bool {
	if atomic.LoadInt32(&r.closed) == 1 {
		return false
	}

	sh := r.getShard(pair)
	sh.Lock()
	defer sh.Unlock()

	if pb, exists := sh.m[id]; exists && pb.Pair == pair {
		delete(sh.m, id)
		atomic.AddInt64(&r.totalCount, -1)
		logger.Debugf("[PENDING_BUY] Removed %s for %s (ID: %s)", pb.CreatedBy, pair, id[:8])
		return true
	}

	return false
}

func (r *shardedRepo) RemoveAll(pair string) int {
	if atomic.LoadInt32(&r.closed) == 1 {
		return 0
	}

	sh := r.getShard(pair)
	sh.Lock()
	defer sh.Unlock()

	removed := 0
	for id, pb := range sh.m {
		if pb.Pair == pair {
			delete(sh.m, id)
			removed++
		}
	}

	atomic.AddInt64(&r.totalCount, -int64(removed))

	if removed > 0 {
		logger.Infof("[PENDING_BUY] Removed all %d entries for %s", removed, pair)
	}

	return removed
}

func (r *shardedRepo) Count() int {
	return int(atomic.LoadInt64(&r.totalCount))
}

func (r *shardedRepo) CountByPair(pair string) int {
	if atomic.LoadInt32(&r.closed) == 1 {
		return 0
	}

	sh := r.getShard(pair)
	sh.RLock()
	defer sh.RUnlock()

	return r.countPairInShard(sh, pair)
}

func (r *shardedRepo) Cleanup() int {
	if atomic.LoadInt32(&r.closed) == 1 {
		return 0
	}

	removed := 0
	now := time.Now()
	maxAge := time.Duration(maxAgeHours) * time.Hour

	for i := range r.shards {
		sh := &r.shards[i]
		sh.Lock()

		for id, pb := range sh.m {
			age := now.Sub(pb.TriggerTime)
			if age > maxAge {
				delete(sh.m, id)
				removed++
				continue
			}
			// smart prune: very old & low score => evict
			if age > 75*time.Minute {
				if r.calculateScore(pb, now) < 0.12 {
					delete(sh.m, id)
					removed++
					continue
				}
			}
		}

		sh.lastCleanup = now
		sh.Unlock()
	}

	atomic.AddInt64(&r.totalCount, -int64(removed))

	if removed > 0 {
		logger.Infof("[PENDING_BUY] Cleanup removed %d stale/low-score entries", removed)
	}

	return removed
}

func (r *shardedRepo) GetStats() *RepoStats {
	if atomic.LoadInt32(&r.closed) == 1 {
		return &RepoStats{}
	}

	stats := &RepoStats{
		PairCounts:   make(map[string]int),
		MarketStates: make(map[string]int),
		PriorityDist: make(map[int]int),
	}

	now := time.Now()
	var totalAge time.Duration
	var totalConfidence float64
	count := 0

	oldest := now
	newest := time.Time{}

	for i := range r.shards {
		sh := &r.shards[i]
		sh.RLock()

		for _, pb := range sh.m {
			count++
			stats.PairCounts[pb.Pair]++
			stats.MarketStates[pb.MarketState.String()]++
			stats.PriorityDist[pb.Priority]++

			totalAge += now.Sub(pb.TriggerTime)
			totalConfidence += pb.ConfidenceScore

			if pb.TriggerTime.Before(oldest) {
				oldest = pb.TriggerTime
			}
			if pb.TriggerTime.After(newest) {
				newest = pb.TriggerTime
			}
		}

		sh.RUnlock()
	}

	stats.TotalCount = count
	if count > 0 {
		stats.AvgAgeMinutes = totalAge.Minutes() / float64(count)
		stats.AvgConfidence = totalConfidence / float64(count)
		stats.OldestEntry = oldest
		stats.NewestEntry = newest
	}

	return stats
}

func (r *shardedRepo) Close() error {
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return fmt.Errorf("repository already closed")
	}

	close(r.stopCh)

	if r.cleanupTicker != nil {
		r.cleanupTicker.Stop()
	}

	for i := range r.shards {
		r.shards[i].Lock()
		r.shards[i].m = make(map[string]*PendingBuy)
		r.shards[i].Unlock()
	}

	atomic.StoreInt64(&r.totalCount, 0)

	logger.Info("[PENDING_BUY] Repository closed")
	return nil
}

func (r *shardedRepo) getShard(pair string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(pair))
	return &r.shards[h.Sum32()&(shardCount-1)]
}

func (r *shardedRepo) generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// calculateScore: regime-aware composite score with long half-life.
// Range: roughly -inf..~5.0, practical ~ -0.5..~4.0; consumers compare relative order.
func (r *shardedRepo) calculateScore(pb *PendingBuy, now time.Time) float64 {
	ageMin := now.Sub(pb.TriggerTime).Minutes()

	// Longer half-life (~28m) so good setups don't die too fast
	recencyMultiplier := math.Exp(math.Log(0.5) * (ageMin / 28.0)) // 0.5^(age/28)

	// RSI preference: buy weakness, avoid knives & heat
	rsi := pb.RsiVal
	rsiScore := 0.0
	switch {
	case rsi <= 18:
		rsiScore = -0.55
	case rsi <= 30:
		rsiScore = 0.30
	case rsi <= 45:
		rsiScore = 0.45
	case rsi <= 60:
		rsiScore = 0.30
	case rsi <= 70:
		rsiScore = 0.05
	default:
		rsiScore = -0.30
	}

	// MACD momentum: (line - signal), capped
	macdDelta := pb.MacdLine - pb.MacdSignal
	macdScore := math.Max(-0.6, math.Min(0.6, macdDelta*3.0))

	// Stochastic context
	stochScore := 0.0
	if pb.StochasticK < 35 && pb.StochasticK > pb.StochasticD {
		stochScore = 0.15
	} else if pb.StochasticK > 85 && pb.StochasticK < pb.StochasticD {
		stochScore = -0.20
	}

	// CCI/MFI sanity
	cciScore := 0.0
	absCCI := math.Abs(pb.CCIValue)
	switch {
	case absCCI <= 50:
		cciScore = 0.10
	case absCCI >= 180:
		cciScore = -0.30
	}
	mfiScore := 0.0
	switch {
	case pb.MFIValue < 15 || pb.MFIValue > 85:
		mfiScore = -0.30
	case pb.MFIValue >= 30 && pb.MFIValue <= 70:
		mfiScore = 0.20
	}

	// Confidence & priority
	confidenceScore := math.Max(0, math.Min(1, pb.ConfidenceScore)) * 1.15
	priorityScore := float64(pb.Priority) * 0.22 // priority 1..5 => 0.22..1.10

	// Volatility: cap to avoid favoring chaos
	volatilityScore := math.Min(pb.ATR*3.2, 0.7)

	// Price position in bands: prefer lower third
	pos := pb.PricePosition // 0..1 (lower..upper)
	posScore := 0.0
	switch {
	case pos <= 0.2:
		posScore = 0.6
	case pos <= 0.4:
		posScore = 0.38
	case pos <= 0.6:
		posScore = 0.12
	case pos <= 0.8:
		posScore = -0.08
	default:
		posScore = -0.28
	}

	// Trend/position synergy
	trendPosScore := 0.0
	if pb.TrendStrength > 0 && pos <= 0.5 {
		trendPosScore = math.Min(0.4, pb.TrendStrength*0.5)
	} else if pb.TrendStrength < 0 && pos > 0.6 {
		trendPosScore = -0.3
	}

	// Volume context if present
	volRatioScore := 0.0
	if pb.VolumeRatio > 0 {
		switch {
		case pb.VolumeRatio >= 1.4:
			volRatioScore = 0.25
		case pb.VolumeRatio >= 1.2:
			volRatioScore = 0.15
		case pb.VolumeRatio <= 0.7:
			volRatioScore = -0.10
		}
	}

	// Observation confidence bonus: confirmed bullish multiple times
	obsBoost := math.Min(0.28, float64(max(0, pb.okCount-1))*0.085)

	// Small freshness boost for very new (first few minutes) to help strong signals win early
	freshBoost := 0.0
	if ageMin <= 5 {
		freshBoost = 0.08
	}

	// State weight
	stateMultiplier := r.getMarketStateMultiplier(pb.MarketState)

	// Sum
	base := rsiScore + macdScore + stochScore + cciScore + mfiScore +
		confidenceScore + priorityScore + volatilityScore + posScore +
		trendPosScore + volRatioScore + obsBoost + freshBoost

	// Final score
	return base * stateMultiplier * recencyMultiplier
}

func (r *shardedRepo) getMarketStateMultiplier(state models.MarketState) float64 {
	switch state {
	case models.StronglyTrending:
		return 2.1
	case models.Trending:
		return 1.6
	case models.Transitional:
		return 1.25
	case models.RangeBound:
		return 1.0
	case models.Chaotic:
		return 0.65
	default:
		return 1.0
	}
}

// Adaptive gates (stricter in chaotic markets, looser in strong trends)

func (r *shardedRepo) adaptiveMinAge(state models.MarketState) time.Duration {
	switch state {
	case models.Chaotic:
		return 6 * time.Minute
	case models.Transitional, models.RangeBound:
		return 4 * time.Minute
	case models.Trending:
		return 3 * time.Minute
	case models.StronglyTrending:
		return 2 * time.Minute
	default:
		return 3 * time.Minute
	}
}

func (r *shardedRepo) adaptiveMinObservations(state models.MarketState) int {
	switch state {
	case models.Chaotic:
		return 4
	case models.Transitional, models.RangeBound:
		return 3
	case models.Trending:
		return 2
	case models.StronglyTrending:
		return 1
	default:
		return 2
	}
}

func (r *shardedRepo) adaptiveEvalSpacing(state models.MarketState) time.Duration {
	switch state {
	case models.Chaotic:
		return 55 * time.Second
	case models.Transitional, models.RangeBound:
		return 45 * time.Second
	case models.Trending:
		return 35 * time.Second
	case models.StronglyTrending:
		return 25 * time.Second
	default:
		return minEvalSpacing
	}
}

func (r *shardedRepo) countPairInShard(sh *shard, pair string) int {
	count := 0
	for _, pb := range sh.m {
		if pb.Pair == pair {
			count++
		}
	}
	return count
}

func (r *shardedRepo) removeWorstInShard(sh *shard, pair string) {
	var worstID string
	worstScore := math.Inf(1)
	now := time.Now()

	for id, pb := range sh.m {
		if pb.Pair == pair {
			score := r.calculateScore(pb, now)
			if score < worstScore {
				worstScore = score
				worstID = id
			}
		}
	}

	if worstID != "" {
		delete(sh.m, worstID)
		atomic.AddInt64(&r.totalCount, -1)
	}
}

func (r *shardedRepo) backgroundCleanup() {
	for {
		select {
		case <-r.stopCh:
			return
		case <-r.cleanupTicker.C:
			if atomic.LoadInt32(&r.closed) == 0 {
				r.Cleanup()
			}
		}
	}
}

// small util
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
