package strategies

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
)

type TrendSnapshot struct {
	Timestamp       time.Time
	Price           float64
	RSI             float64
	MacdLine        float64
	MacdSignal      float64
	HistSlope       float64
	RsiSlope        float64
	MiddleBandSlope float64
	Volume          float64
}

type TrendHistory struct {
	Snapshots      []TrendSnapshot
	MaxSnapshots   int // rolling window size
	TrendScore     float64
	TrendDirection string  // "UP", "DOWN", "SIDEWAYS"
	Consistency    float64 // 0.0-1.0, higher = more consistent
	Acceleration   float64 // rate of change improvement
}

type PendingBuy struct {
	ID              string
	Pair            string
	TriggerPrice    float64
	TriggerTime     time.Time
	RsiVal          float64
	MacdLine        float64
	MacdSignal      float64
	MarketState     models.MarketState
	ATR             float64
	BollingerWidth  float64
	StochasticK     float64
	StochasticD     float64
	CCIValue        float64
	MFIValue        float64
	TrendStrength   float64
	PricePosition   float64
	VolumeRatio     float64
	ConfidenceScore float64
	Priority        int
	CreatedBy       string

	TrendHistory *TrendHistory
	FirstSeen    time.Time
	LastUpdate   time.Time
	UpdateCount  int
}

type PendingBuyRepo interface {
	Add(pb *PendingBuy) error
	AddOrReplace(pb *PendingBuy) (*PendingBuy, error)
	Remove(pair string, id string) bool
	RemoveAll(pair string)
	GetAll(pair string) []*PendingBuy
	Exists(pair string) bool
	ExistsWithCondition(pair string, cond func(*PendingBuy) bool) bool
	Confirm(pair string, validator func(*PendingBuy) bool) *PendingBuy
	Count() int

	UpdateSnapshot(pair string, ci CurrentIndicators, currentPrice float64) bool
}

type inMemoryPendingBuyRepo struct {
	mu      sync.RWMutex
	entries map[string][]*PendingBuy // keyed by pair
	nextID  int
}

func NewPendingBuyRepo() PendingBuyRepo {
	return &inMemoryPendingBuyRepo{
		entries: make(map[string][]*PendingBuy),
		nextID:  1,
	}
}

func calculatePriceDiff(price, basePrice float64) float64 {
	if basePrice == 0 {
		return 0
	}
	return (price - basePrice) / basePrice
}

func (r *inMemoryPendingBuyRepo) Add(pb *PendingBuy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if pb.ID == "" {
		pb.ID = r.generateID()
	}

	if pb.TrendHistory == nil {
		pb.TrendHistory = &TrendHistory{
			Snapshots:    make([]TrendSnapshot, 0, 10),
			MaxSnapshots: 10, // keep last 10 snapshots
		}
		pb.FirstSeen = time.Now()
	}
	pb.LastUpdate = time.Now()
	pb.UpdateCount = 0

	r.entries[pb.Pair] = append(r.entries[pb.Pair], pb)
	return nil
}

func (r *inMemoryPendingBuyRepo) AddOrReplace(pb *PendingBuy) (*PendingBuy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.entries[pb.Pair]
	var replaced *PendingBuy

	for i, existing := range list {
		priceDiff := calculatePriceDiff(pb.TriggerPrice, existing.TriggerPrice)

		if math.Abs(priceDiff) < 0.005 { // within 0.5%
			replaced = existing

			if existing.TrendHistory != nil {
				pb.TrendHistory = existing.TrendHistory
				pb.FirstSeen = existing.FirstSeen
				pb.UpdateCount = existing.UpdateCount
			}

			list[i] = pb
			r.entries[pb.Pair] = list
			return replaced, nil
		}
	}

	if pb.ID == "" {
		pb.ID = r.generateID()
	}

	if pb.TrendHistory == nil {
		pb.TrendHistory = &TrendHistory{
			Snapshots:    make([]TrendSnapshot, 0, 10),
			MaxSnapshots: 10,
		}
		pb.FirstSeen = time.Now()
	}
	pb.LastUpdate = time.Now()
	pb.UpdateCount = 0

	r.entries[pb.Pair] = append(list, pb)
	return nil, nil
}

func (r *inMemoryPendingBuyRepo) UpdateSnapshot(pair string, ci CurrentIndicators, currentPrice float64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.entries[pair]
	if len(list) == 0 {
		return false
	}

	updated := false
	for _, pb := range list {
		if pb.TrendHistory == nil {
			continue
		}

		snapshot := TrendSnapshot{
			Timestamp:       time.Now(),
			Price:           currentPrice,
			RSI:             ci.RSIVal,
			MacdLine:        ci.MacdLine,
			MacdSignal:      ci.SignalLine,
			HistSlope:       ci.HistSlope,
			RsiSlope:        ci.RsiSlope,
			MiddleBandSlope: ci.MiddleBandSlope,
			Volume:          0, // filled from candles if available
		}

		if len(ci.CandleSticks) > 0 {
			snapshot.Volume = ci.CandleSticks[len(ci.CandleSticks)-1].Volume
		}

		pb.TrendHistory.Snapshots = append(pb.TrendHistory.Snapshots, snapshot)
		if len(pb.TrendHistory.Snapshots) > pb.TrendHistory.MaxSnapshots {
			pb.TrendHistory.Snapshots = pb.TrendHistory.Snapshots[1:]
		}

		r.calculateTrendMetrics(pb.TrendHistory)

		pb.LastUpdate = time.Now()
		pb.UpdateCount++
		updated = true

		logger.DebugColorf(logger.Blue,
			"[TREND SNAPSHOT] %s | Updates:%d | Score:%.2f | Direction:%s | Consistency:%.2f | Accel:%.3f",
			pair, pb.UpdateCount, pb.TrendHistory.TrendScore, pb.TrendHistory.TrendDirection,
			pb.TrendHistory.Consistency, pb.TrendHistory.Acceleration)
	}

	return updated
}

func (r *inMemoryPendingBuyRepo) calculateTrendMetrics(th *TrendHistory) {
	snapshots := th.Snapshots
	n := len(snapshots)

	if n < 2 {
		th.TrendScore = 0.5
		th.TrendDirection = "UNKNOWN"
		th.Consistency = 0.0
		th.Acceleration = 0.0
		return
	}

	priceUps := 0
	priceDowns := 0
	for i := 1; i < n; i++ {
		if snapshots[i].Price > snapshots[i-1].Price {
			priceUps++
		} else if snapshots[i].Price < snapshots[i-1].Price {
			priceDowns++
		}
	}
	priceConsistency := float64(priceUps) / float64(n-1)

	positiveHistSlopes := 0
	for _, s := range snapshots {
		if s.HistSlope > 0 {
			positiveHistSlopes++
		}
	}
	momentumConsistency := float64(positiveHistSlopes) / float64(n)

	rsiImproving := 0
	rsiHealthy := 0
	for i := 1; i < n; i++ {
		if snapshots[i].RSI > snapshots[i-1].RSI {
			rsiImproving++
		}
		if snapshots[i].RSI > 40 && snapshots[i].RSI < 70 {
			rsiHealthy++
		}
	}
	rsiScore := (float64(rsiImproving)/float64(n-1) + float64(rsiHealthy)/float64(n)) / 2.0

	volumeIncreasing := 0
	volumeScore := 0.5
	if n >= 3 {
		for i := 2; i < n; i++ {
			if snapshots[i].Volume > snapshots[i-1].Volume {
				volumeIncreasing++
			}
		}

		volumeScore = float64(volumeIncreasing) / float64(n-2)
	}

	acceleration := 0.0
	if n >= 3 {
		recentHistAvg := 0.0
		olderHistAvg := 0.0
		mid := n / 2

		for i := mid; i < n; i++ {
			recentHistAvg += snapshots[i].HistSlope
		}
		recentHistAvg /= float64(n - mid)

		for i := 0; i < mid; i++ {
			olderHistAvg += snapshots[i].HistSlope
		}
		olderHistAvg /= float64(mid)

		acceleration = recentHistAvg - olderHistAvg
	}

	th.Consistency = (priceConsistency + momentumConsistency + rsiScore) / 3.0
	th.TrendScore = (priceConsistency*0.30 + momentumConsistency*0.35 + rsiScore*0.20 + volumeScore*0.15)
	th.Acceleration = acceleration

	if priceConsistency > 0.65 && momentumConsistency > 0.60 {
		th.TrendDirection = "UP"
	} else if priceConsistency < 0.35 && momentumConsistency < 0.40 {
		th.TrendDirection = "DOWN"
	} else {
		th.TrendDirection = "SIDEWAYS"
	}
}

func (r *inMemoryPendingBuyRepo) Remove(pair string, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.entries[pair]
	for i, pb := range list {
		if pb.ID == id {
			r.entries[pair] = append(list[:i], list[i+1:]...)
			if len(r.entries[pair]) == 0 {
				delete(r.entries, pair)
			}
			return true
		}
	}
	return false
}

func (r *inMemoryPendingBuyRepo) RemoveAll(pair string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, pair)
}

func (r *inMemoryPendingBuyRepo) GetAll(pair string) []*PendingBuy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.entries[pair]
	result := make([]*PendingBuy, len(list))
	copy(result, list)
	return result
}

func (r *inMemoryPendingBuyRepo) Exists(pair string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries[pair]) > 0
}

func (r *inMemoryPendingBuyRepo) ExistsWithCondition(pair string, cond func(*PendingBuy) bool) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, pb := range r.entries[pair] {
		if cond(pb) {
			return true
		}
	}
	return false
}

func (r *inMemoryPendingBuyRepo) Confirm(pair string, validator func(*PendingBuy) bool) *PendingBuy {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.entries[pair]
	for i, pb := range list {
		if validator(pb) {

			r.entries[pair] = append(list[:i], list[i+1:]...)
			if len(r.entries[pair]) == 0 {
				delete(r.entries, pair)
			}

			trendScore := 0.0
			trendDir := "UNKNOWN"
			consistency := 0.0
			if pb.TrendHistory != nil {
				trendScore = pb.TrendHistory.TrendScore
				trendDir = pb.TrendHistory.TrendDirection
				consistency = pb.TrendHistory.Consistency
			}

			logger.InfoColorf(logger.Green,
				"[PENDING CONFIRMED] %s | Age:%.1fm | Updates:%d | TrendScore:%.2f | Direction:%s | Consistency:%.2f",
				pair, time.Since(pb.FirstSeen).Minutes(), pb.UpdateCount,
				trendScore, trendDir, consistency)

			return pb
		}
	}
	return nil
}

func (r *inMemoryPendingBuyRepo) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, list := range r.entries {
		total += len(list)
	}
	return total
}

func (r *inMemoryPendingBuyRepo) generateID() string {
	id := r.nextID
	r.nextID++
	return fmt.Sprintf("%s-%06d", time.Now().Format("20060102-150405"), id)
}

func (pb *PendingBuy) GetTrendQuality() float64 {
	if pb.TrendHistory == nil || len(pb.TrendHistory.Snapshots) < 3 {
		return 0.5 // neutral - not enough data
	}

	th := pb.TrendHistory

	quality := th.TrendScore*0.50 + th.Consistency*0.30

	if th.Acceleration > 0 {
		quality += th.Acceleration * 0.10
		if quality > 1.0 {
			quality = 1.0
		}
	}

	age := time.Since(pb.FirstSeen).Minutes()
	timeFactor := 1.0
	switch {
	case age < 2:
		timeFactor = 0.85
	case age >= 2 && age < 5:
		timeFactor = 0.95
	case age >= 5 && age < 10:
		timeFactor = 1.0
	case age >= 10:
		timeFactor = 0.90
	}

	quality *= timeFactor

	if pb.UpdateCount >= 5 {
		quality += 0.05
	}
	if pb.UpdateCount >= 8 {
		quality += 0.05
	}

	if quality > 1.0 {
		quality = 1.0
	}

	return quality
}

func (pb *PendingBuy) ShouldBuyNow(currentPrice float64, ci CurrentIndicators) bool {
	if pb.TrendHistory == nil || len(pb.TrendHistory.Snapshots) < 3 {
		return false
	}

	th := pb.TrendHistory
	age := time.Since(pb.FirstSeen).Minutes()

	minObservation := 2.0
	switch pb.MarketState {
	case models.StronglyTrending:
		minObservation = 1.5
	case models.Trending:
		minObservation = 2.0
	case models.Chaotic:
		minObservation = 1.0
	case models.Transitional:
		minObservation = 3.0
	default:
		minObservation = 2.5
	}

	if age < minObservation {
		logger.DebugColorf(logger.Yellow, "[TREND WAIT] %s | Age:%.1fm < Min:%.1fm",
			pb.Pair, age, minObservation)
		return false
	}

	trendQuality := pb.GetTrendQuality()
	minQuality := 0.65

	switch pb.MarketState {
	case models.StronglyTrending:
		minQuality = 0.70
	case models.Trending:
		minQuality = 0.65
	case models.Chaotic:
		minQuality = 0.75
	case models.Transitional:
		minQuality = 0.68
	default:
		minQuality = 0.65
	}

	if trendQuality < minQuality {
		logger.DebugColorf(logger.Yellow, "[TREND QUALITY LOW] %s | Quality:%.2f < Min:%.2f",
			pb.Pair, trendQuality, minQuality)
		return false
	}

	if th.TrendDirection != "UP" {
		logger.DebugColorf(logger.Yellow, "[TREND DIRECTION WRONG] %s | Direction:%s",
			pb.Pair, th.TrendDirection)
		return false
	}

	if th.Consistency < 0.60 {
		logger.DebugColorf(logger.Yellow, "[TREND INCONSISTENT] %s | Consistency:%.2f < 0.60",
			pb.Pair, th.Consistency)
		return false
	}

	if len(th.Snapshots) > 0 {
		latest := th.Snapshots[len(th.Snapshots)-1]
		if latest.HistSlope <= 0 || latest.MacdLine <= latest.MacdSignal {
			logger.DebugColorf(logger.Yellow, "[TREND MOMENTUM WEAK] %s | HistSlope:%.6f, MACD cross invalid",
				pb.Pair, latest.HistSlope)
			return false
		}
	}

	priceMove := (currentPrice - pb.TriggerPrice) / pb.TriggerPrice
	maxChase := 0.020
	switch pb.MarketState {
	case models.StronglyTrending:
		maxChase = 0.030
	case models.Chaotic:
		maxChase = 0.015
	}

	if priceMove > maxChase {
		logger.DebugColorf(logger.Yellow, "[TREND CHASE TOO FAR] %s | Move:%.2f%% > Max:%.2f%%",
			pb.Pair, priceMove*100, maxChase*100)
		return false
	}

	logger.InfoColorf(logger.Green,
		"[TREND BUY READY ✓] %s | Quality:%.2f | Dir:%s | Consist:%.2f | Accel:%.3f | Age:%.1fm | Updates:%d",
		pb.Pair, trendQuality, th.TrendDirection, th.Consistency, th.Acceleration, age, pb.UpdateCount)

	return true
}
