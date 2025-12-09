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

	EMA20      float64
	EMA50      float64
	EMA200     float64
	EMASlope20 float64
	EMASlope50 float64

	ADX     float64
	PlusDI  float64
	MinusDI float64

	KeltnerMid   float64
	KeltnerUpper float64
	KeltnerLower float64
	KeltnerSlope float64
	BandPosition float64
}

type TrendHistory struct {
	Snapshots      []TrendSnapshot
	MaxSnapshots   int
	TrendScore     float64
	TrendDirection string
	Consistency    float64
	Acceleration   float64
}

type MarketMemoryPoint struct {
	Timestamp   time.Time
	Price       float64
	HistSlope   float64
	RsiSlope    float64
	ADX         float64
	PlusDI      float64
	MinusDI     float64
	MarketState models.MarketState
}

type MarketTrendMemory struct {
	Points    []MarketMemoryPoint
	MaxPoints int
}

type TrendBias struct {
	Direction  string
	Strength   float64
	Samples    int
	LastUpdate time.Time
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

	UpdateSnapshot(pair string, ci CurrentIndicators, currentPrice float64, state models.MarketState) bool
	RecordMarketTrend(pair string, state models.MarketState, ci CurrentIndicators, currentPrice float64)
	GetTrendBias(pair string) TrendBias
}

type inMemoryPendingBuyRepo struct {
	mu      sync.RWMutex
	entries map[string][]*PendingBuy // keyed by pair
	memory  map[string]*MarketTrendMemory
	nextID  int
}

func NewPendingBuyRepo() PendingBuyRepo {
	return &inMemoryPendingBuyRepo{
		entries: make(map[string][]*PendingBuy),
		memory:  make(map[string]*MarketTrendMemory),
		nextID:  1,
	}
}

func calculatePriceDiff(price, basePrice float64) float64 {
	if basePrice == 0 {
		return 0
	}
	return (price - basePrice) / basePrice
}

func clamp01(val float64) float64 {
	switch {
	case val < 0:
		return 0
	case val > 1:
		return 1
	default:
		return val
	}
}

func marketStateWeight(st models.MarketState) float64 {
	switch st {
	case models.StronglyTrending:
		return 1.2
	case models.Trending:
		return 1.0
	case models.Transitional:
		return 0.75
	case models.RangeBound:
		return 0.65
	case models.Chaotic:
		return 0.35
	case models.Default:
		return 0.6
	default:
		return 0.6
	}
}

func (r *inMemoryPendingBuyRepo) Add(pb *PendingBuy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if pb.ID == "" {
		pb.ID = r.generateID()
	}

	if pb.TrendHistory == nil {
		pb.TrendHistory = &TrendHistory{
			Snapshots:    make([]TrendSnapshot, 0, 100),
			MaxSnapshots: 100,
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

		if math.Abs(priceDiff) < 0.005 {
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

func (m *MarketTrendMemory) add(point MarketMemoryPoint) {
	if m.MaxPoints <= 0 {
		m.MaxPoints = 240
	}
	m.Points = append(m.Points, point)

	cutoff := time.Now().Add(-12 * time.Hour)
	trimIdx := 0
	for i, pt := range m.Points {
		if pt.Timestamp.After(cutoff) {
			break
		}
		trimIdx = i + 1
	}
	if trimIdx > 0 && trimIdx < len(m.Points) {
		m.Points = m.Points[trimIdx:]
	} else if trimIdx >= len(m.Points) {
		m.Points = m.Points[len(m.Points)-1:]
	}

	if len(m.Points) > m.MaxPoints {
		m.Points = m.Points[len(m.Points)-m.MaxPoints:]
	}
}

func (m *MarketTrendMemory) bias() TrendBias {
	if m == nil || len(m.Points) == 0 {
		return TrendBias{Direction: "UNKNOWN"}
	}

	var (
		upVotesW   float64
		downVotesW float64
		adxSumW    float64
		weightSum  float64
	)

	for _, pt := range m.Points {
		w := marketStateWeight(pt.MarketState)
		if w <= 0 {
			continue
		}

		switch {
		case pt.HistSlope > 0:
			upVotesW += 1 * w
		case pt.HistSlope < 0:
			downVotesW += 1 * w
		}

		switch {
		case pt.RsiSlope > 0:
			upVotesW += 1 * w
		case pt.RsiSlope < 0:
			downVotesW += 1 * w
		}

		switch {
		case pt.PlusDI > pt.MinusDI:
			upVotesW += 1 * w
		case pt.PlusDI < pt.MinusDI:
			downVotesW += 1 * w
		}

		adxSumW += pt.ADX * w
		weightSum += w
	}

	totalVotes := upVotesW + downVotesW
	if totalVotes == 0 {
		return TrendBias{Direction: "UNKNOWN"}
	}

	dirScore := upVotesW / totalVotes

	direction := "SIDEWAYS"
	switch {
	case dirScore >= 0.58:
		direction = "UP"
	case dirScore <= 0.42:
		direction = "DOWN"
	}

	if weightSum == 0 {
		weightSum = 1
	}
	avgADX := adxSumW / weightSum
	adxScore := clamp01(avgADX / 35.0)

	strength := clamp01(dirScore*0.55 + adxScore*0.45)

	return TrendBias{
		Direction:  direction,
		Strength:   strength,
		Samples:    len(m.Points),
		LastUpdate: m.Points[len(m.Points)-1].Timestamp,
	}
}

func (r *inMemoryPendingBuyRepo) recordTrendMemoryLocked(pair string, state models.MarketState, ci CurrentIndicators, currentPrice float64) {
	if state == models.Chaotic && ci.ADX < 12 {
		return
	}

	mem, ok := r.memory[pair]
	if !ok {
		mem = &MarketTrendMemory{MaxPoints: 240}
		r.memory[pair] = mem
	}

	mem.add(MarketMemoryPoint{
		Timestamp:   time.Now(),
		Price:       currentPrice,
		HistSlope:   ci.HistSlope,
		RsiSlope:    ci.RsiSlope,
		ADX:         ci.ADX,
		PlusDI:      ci.PlusDI,
		MinusDI:     ci.MinusDI,
		MarketState: state,
	})
}
func (r *inMemoryPendingBuyRepo) RecordMarketTrend(pair string, state models.MarketState, ci CurrentIndicators, currentPrice float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordTrendMemoryLocked(pair, state, ci, currentPrice)
}

func (r *inMemoryPendingBuyRepo) GetTrendBias(pair string) TrendBias {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if mem, ok := r.memory[pair]; ok {
		return mem.bias()
	}
	return TrendBias{Direction: "UNKNOWN"}
}

func (r *inMemoryPendingBuyRepo) UpdateSnapshot(pair string, ci CurrentIndicators, currentPrice float64, state models.MarketState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.entries[pair]
	if len(list) == 0 {
		return false
	}

	r.recordTrendMemoryLocked(pair, state, ci, currentPrice)

	updated := false

	for _, pb := range list {
		if pb.TrendHistory == nil {
			continue
		}

		kMid := ci.KCMid
		kUpper := ci.KCUpper
		kLower := ci.KCLower

		bandPos := 0.5
		if span := kUpper - kLower; span > 0 {
			bandPos = (currentPrice - kLower) / span
			if bandPos < 0 {
				bandPos = 0
			} else if bandPos > 1 {
				bandPos = 1
			}
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
			Volume: func() float64 {
				if len(ci.CandleSticks) > 0 {
					return ci.CandleSticks[len(ci.CandleSticks)-1].Volume
				}
				return 0
			}(),

			EMA20:      ci.EMA20,
			EMA50:      ci.EMA50,
			EMA200:     ci.EMA200,
			EMASlope20: ci.EMASlope20,
			EMASlope50: ci.EMASlope50,

			ADX:     ci.ADX,
			PlusDI:  ci.PlusDI,
			MinusDI: ci.MinusDI,

			KeltnerMid:   kMid,
			KeltnerUpper: kUpper,
			KeltnerLower: kLower,
			BandPosition: bandPos,
		}

		th := pb.TrendHistory
		if len(th.Snapshots) > 0 {
			prev := th.Snapshots[len(th.Snapshots)-1]
			snapshot.KeltnerSlope = snapshot.KeltnerMid - prev.KeltnerMid
		}

		th.Snapshots = append(th.Snapshots, snapshot)
		if len(th.Snapshots) > th.MaxSnapshots {
			th.Snapshots = th.Snapshots[1:]
		}

		r.calculateTrendMetrics(th)

		pb.LastUpdate = time.Now()
		pb.UpdateCount++
		updated = true

		latest := th.Snapshots[len(th.Snapshots)-1]

		logger.DebugColorf(
			logger.Blue,
			"[TREND SNAPSHOT] %s | Updates:%d | Score:%.2f | Dir:%s | Consistency:%.2f | Accel:%.3f | ADX:%.1f",
			pair, pb.UpdateCount, th.TrendScore, th.TrendDirection,
			th.Consistency, th.Acceleration, latest.ADX,
		)
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
		switch {
		case snapshots[i].Price > snapshots[i-1].Price:
			priceUps++
		case snapshots[i].Price < snapshots[i-1].Price:
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

	kcUp := 0
	kcDown := 0
	for i := 1; i < n; i++ {
		d := snapshots[i].KeltnerMid - snapshots[i-1].KeltnerMid
		switch {
		case d > 0:
			kcUp++
		case d < 0:
			kcDown++
		}
	}
	kcSlopeScore := float64(kcUp) / float64(n-1)

	adxTrendCount := 0
	adxBearCount := 0
	const adxThresh = 18.0
	var adxSum float64
	for _, s := range snapshots {
		adxSum += s.ADX
		if s.ADX >= adxThresh {
			if s.PlusDI > s.MinusDI {
				adxTrendCount++
			} else if s.PlusDI < s.MinusDI {
				adxBearCount++
			}
		}
	}

	upTrendBars := 0
	downTrendBars := 0
	for _, s := range snapshots {
		if s.ADX >= 18 && s.PlusDI > s.MinusDI {
			upTrendBars++
		}
		if s.ADX >= 18 && s.MinusDI > s.PlusDI {
			downTrendBars++
		}
	}
	upStrength := float64(upTrendBars) / float64(n)
	downStrength := float64(downTrendBars) / float64(n)

	directionalStrength := upStrength

	macdAbove := 0
	for _, s := range snapshots {
		if s.MacdLine > s.MacdSignal {
			macdAbove++
		}
	}
	macdAboveScore := float64(macdAbove) / float64(n)

	emaStack := 0
	emaSlopePos := 0
	for _, s := range snapshots {
		if s.EMA20 >= s.EMA50 && s.EMA50 >= s.EMA200 {
			emaStack++
		}
		if s.EMASlope20 > 0 && s.EMASlope50 > 0 {
			emaSlopePos++
		}
	}
	emaStackScore := float64(emaStack) / float64(n)
	emaSlopeScore := float64(emaSlopePos) / float64(n)

	acceleration := 0.0
	if n >= 3 {
		mid := n / 2
		recentHistAvg := 0.0
		olderHistAvg := 0.0

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

	th.Consistency = priceConsistency*0.30 +
		momentumConsistency*0.25 +
		rsiScore*0.15 +
		directionalStrength*0.15 +
		emaSlopeScore*0.10 +
		macdAboveScore*0.05

	th.TrendScore = priceConsistency*0.25 +
		momentumConsistency*0.25 +
		rsiScore*0.15 +
		volumeScore*0.10 +
		directionalStrength*0.15 +
		emaSlopeScore*0.05 +
		emaStackScore*0.05
	th.Acceleration = acceleration

	avgADX := adxSum / float64(n)
	strongTrend := avgADX >= 25 && adxTrendCount >= adxBearCount
	upDirScore := priceConsistency*0.35 +
		momentumConsistency*0.25 +
		kcSlopeScore*0.15 +
		emaSlopeScore*0.10 +
		macdAboveScore*0.10 +
		emaStackScore*0.05

	downDirScore := downStrength*0.40 +
		(1-priceConsistency)*0.30 +
		(1-momentumConsistency)*0.20 +
		kcSlopeScore*0.10

	switch {
	case strongTrend && upDirScore >= 0.52:
		th.TrendDirection = "UP"
	case upDirScore >= 0.60 && upStrength > 0.50:
		th.TrendDirection = "UP"
	case downDirScore >= 0.55 && adxBearCount >= adxTrendCount:
		th.TrendDirection = "DOWN"
	case downStrength > 0.55 && priceConsistency < 0.45:
		th.TrendDirection = "DOWN"
	default:
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
			accel := 0.0
			if pb.TrendHistory != nil {
				trendScore = pb.TrendHistory.TrendScore
				trendDir = pb.TrendHistory.TrendDirection
				consistency = pb.TrendHistory.Consistency
				accel = pb.TrendHistory.Acceleration
			}

			logger.InfoColorf(
				logger.Green,
				"[PENDING CONFIRMED] %s | Age:%.1fm | Updates:%d | TrendScore:%.2f | Dir:%s | Consist:%.2f | Accel:%.3f",
				pair, time.Since(pb.FirstSeen).Minutes(), pb.UpdateCount,
				trendScore, trendDir, consistency, accel,
			)

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
	}

	age := time.Since(pb.FirstSeen).Minutes()
	timeFactor := 1.0
	switch {
	case age < 2:
		timeFactor = 0.85
	case age >= 2 && age < 5:
		timeFactor = 0.95
	case age >= 5 && age < 12:
		timeFactor = 1.0
	case age >= 12 && age < 25:
		timeFactor = 0.93
	case age >= 25:
		timeFactor = 0.88
	}
	quality *= timeFactor

	if pb.UpdateCount >= 5 {
		quality += 0.04
	}
	if pb.UpdateCount >= 8 {
		quality += 0.04
	}

	if quality > 1.0 {
		quality = 1.0
	} else if quality < 0 {
		quality = 0
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
		logger.DebugColorf(
			logger.Yellow,
			"[TREND WAIT] %s | Age:%.1fm < Min:%.1fm",
			pb.Pair, age, minObservation,
		)
		return false
	}

	adx := ci.ADX
	plus := ci.PlusDI
	minus := ci.MinusDI

	const globalADXMin = 8.0
	if adx < globalADXMin {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND ADX TOO LOW] %s | ADX=%.1f < %.1f",
			pb.Pair, adx, globalADXMin,
		)
		return false
	}

	var minTrendADX float64
	switch pb.MarketState {
	case models.StronglyTrending:
		minTrendADX = 14.0
	case models.Trending:
		minTrendADX = 12.0
	default:
		minTrendADX = 10.0
	}

	if (pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending) && adx < minTrendADX {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND DMI WEAK] %s | State:%s ADX=%.1f < MinTrend=%.1f",
			pb.Pair, pb.MarketState.String(), adx, minTrendADX,
		)
		return false
	}

	if (pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending) && plus <= minus {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND DMI DIR WEAK] %s | State:%s ADX=%.1f, +DI=%.1f <= -DI=%.1f",
			pb.Pair, pb.MarketState.String(), adx, plus, minus,
		)
		return false
	}

	trendQuality := pb.GetTrendQuality()
	minQuality := 0.65

	switch pb.MarketState {
	case models.StronglyTrending:
		minQuality = 0.68
	case models.Trending:
		minQuality = 0.65
	case models.Chaotic:
		minQuality = 0.75
	case models.Transitional:
		minQuality = 0.68
	default:
		minQuality = 0.65
	}

	if adx >= 28 {
		minQuality -= 0.04
	} else if adx < 16 {
		minQuality += 0.04
	}
	if minQuality < 0.55 {
		minQuality = 0.55
	}
	if minQuality > 0.85 {
		minQuality = 0.85
	}

	if trendQuality < minQuality {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND QUALITY LOW] %s | Quality:%.2f < Min:%.2f",
			pb.Pair, trendQuality, minQuality,
		)
		return false
	}

	if th.TrendDirection != "UP" {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND DIRECTION WRONG] %s | Direction:%s",
			pb.Pair, th.TrendDirection,
		)
		return false
	}

	if th.Consistency < 0.60 {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND INCONSISTENT] %s | Consistency:%.2f < 0.60",
			pb.Pair, th.Consistency,
		)
		return false
	}

	latest := th.Snapshots[len(th.Snapshots)-1]

	if latest.HistSlope <= 0 || latest.MacdLine <= latest.MacdSignal {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND MOMENTUM WEAK] %s | HistSlope:%.6f, MACD cross invalid",
			pb.Pair, latest.HistSlope,
		)
		return false
	}

	if ci.RsiSlope < 0 && latest.HistSlope < 0.00005 {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND MOMENTUM ROLLING] %s | RsiSlope:%.6f, HistSlope:%.6f",
			pb.Pair, ci.RsiSlope, latest.HistSlope,
		)
		return false
	}

	if ci.RSIVal >= 70 {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND RSI OVERBOUGHT] %s | RSI:%.2f",
			pb.Pair, ci.RSIVal,
		)
		return false
	}

	baseADXMin := 16.0
	if pb.MarketState == models.StronglyTrending {
		baseADXMin = 14.0
	}

	if latest.ADX < baseADXMin || latest.PlusDI <= latest.MinusDI {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND ADX/DI WEAK] %s | ADX:%.1f (min:%.1f), +DI:%.1f, -DI:%.1f",
			pb.Pair, latest.ADX, baseADXMin, latest.PlusDI, latest.MinusDI,
		)
		return false
	}

	hasKC := latest.KeltnerUpper > 0 && latest.KeltnerLower > 0
	if hasKC {
		if latest.KeltnerSlope <= 0 {
			logger.DebugColorf(
				logger.Yellow,
				"[TREND KC SLOPE DOWN] %s | KeltnerSlope:%.6f",
				pb.Pair, latest.KeltnerSlope,
			)
			return false
		}

		if currentPrice >= latest.KeltnerUpper*1.01 {
			logger.DebugColorf(
				logger.Yellow,
				"[TREND OVEREXTENDED KC] %s | cp=%.4f >= KU*1.01=%.4f",
				pb.Pair, currentPrice, latest.KeltnerUpper*1.01,
			)
			return false
		}
	} else {
		logger.DebugColorf(
			logger.BrightBlack,
			"[TREND KC MISSING] %s | proceeding without KC guards",
			pb.Pair,
		)
	}

	if th.Acceleration < -0.00005 {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND DECELERATING] %s | Accel:%.6f",
			pb.Pair, th.Acceleration,
		)
		return false
	}

	emaSlopeOK := latest.EMASlope20 > 0 && latest.EMASlope50 > 0
	if !emaSlopeOK && !(ci.ADX >= 28 && th.Acceleration > 0) {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND EMA SLOPE WEAK] %s | EMA20s:%.6f EMA50s:%.6f",
			pb.Pair, latest.EMASlope20, latest.EMASlope50,
		)
		return false
	}

	priceMove := (currentPrice - pb.TriggerPrice) / pb.TriggerPrice
	maxChase := 0.020
	switch pb.MarketState {
	case models.StronglyTrending:
		maxChase = 0.030
	case models.Chaotic:
		maxChase = 0.015
	default:
		maxChase = 0.020
	}
	if priceMove > maxChase {
		logger.DebugColorf(
			logger.Yellow,
			"[TREND CHASE TOO FAR] %s | Move:%.2f%% > Max:%.2f%%",
			pb.Pair, priceMove*100, maxChase*100,
		)
		return false
	}

	logger.InfoColorf(
		logger.Green,
		"[TREND BUY READY ✓] %s | Quality:%.2f | Dir:%s | Consist:%.2f | Accel:%.3f | Age:%.1fm | Updates:%d",
		pb.Pair, trendQuality, th.TrendDirection, th.Consistency, th.Acceleration, age, pb.UpdateCount,
	)

	return true
}
