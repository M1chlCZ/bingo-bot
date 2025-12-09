package strategies

import (
	"fmt"
	"sync"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
)

type EquityGuardConfig struct {
	MaxLookbackTrades int
	MaxLosingStreak   int
	MinTradesForBlock int
	BlockDuration     time.Duration
	MinAvgPnLForBlock float64
}

type EquitySample struct {
	Time  time.Time
	PnL   float64
	Label string
}

type EquityStats struct {
	Trades         []EquitySample
	LosingStreak   int
	LastBlockUntil time.Time
}

type EquityGuard struct {
	mu      sync.Mutex
	cfg     EquityGuardConfig
	perPair map[string]*EquityStats
}

func NewEquityGuardDefault() *EquityGuard {
	return &EquityGuard{
		cfg: EquityGuardConfig{
			MaxLookbackTrades: 25,
			MaxLosingStreak:   4,
			MinTradesForBlock: 6,
			BlockDuration:     2 * time.Hour,
			MinAvgPnLForBlock: -0.25,
		},
		perPair: make(map[string]*EquityStats),
	}
}

func (eg *EquityGuard) get(pair string) *EquityStats {
	s, ok := eg.perPair[pair]
	if !ok {
		s = &EquityStats{
			Trades:       make([]EquitySample, 0, eg.cfg.MaxLookbackTrades),
			LosingStreak: 0,
		}
		eg.perPair[pair] = s
	}
	return s
}

func (eg *EquityGuard) RecordTrade(pair string, pnlPercent float64, label string) {
	if eg == nil {
		return
	}

	now := time.Now()

	eg.mu.Lock()
	defer eg.mu.Unlock()

	stats := eg.get(pair)

	stats.Trades = append(stats.Trades, EquitySample{
		Time:  now,
		PnL:   pnlPercent,
		Label: label,
	})
	if len(stats.Trades) > eg.cfg.MaxLookbackTrades {
		stats.Trades = stats.Trades[len(stats.Trades)-eg.cfg.MaxLookbackTrades:]
	}

	if pnlPercent < 0 {
		stats.LosingStreak++
	} else {
		stats.LosingStreak = 0
	}

	if len(stats.Trades) >= eg.cfg.MinTradesForBlock {
		avg, wins, losses := eg.computeStatsLocked(stats)

		if (stats.LosingStreak >= eg.cfg.MaxLosingStreak || avg <= eg.cfg.MinAvgPnLForBlock) &&
			now.After(stats.LastBlockUntil) {

			stats.LastBlockUntil = now.Add(eg.cfg.BlockDuration)
			logger.InfoColorf(
				logger.Red,
				"[EQUITY-GUARD TRIGGER] %s => block %v (losingStreak=%d, avgPnL=%.2f%%, wins=%d, losses=%d, lastExit=%s)",
				pair,
				eg.cfg.BlockDuration,
				stats.LosingStreak,
				avg,
				wins,
				losses,
				now.Format(time.RFC3339),
			)
		}
	}
}

func (eg *EquityGuard) ShouldBlockNewEntry(pair string) (bool, string) {
	if eg == nil {
		return false, ""
	}

	now := time.Now()

	eg.mu.Lock()
	defer eg.mu.Unlock()

	stats, ok := eg.perPair[pair]
	if !ok || len(stats.Trades) == 0 {
		return false, ""
	}

	if stats.LastBlockUntil.After(now) {
		avg, wins, losses := eg.computeStatsLocked(stats)
		reason := fmt.Sprintf(
			"equity cooldown until %s (losingStreak=%d, avgPnL=%.2f%%, wins=%d, losses=%d)",
			stats.LastBlockUntil.Format(time.RFC3339),
			stats.LosingStreak,
			avg,
			wins,
			losses,
		)
		return true, reason
	}

	return false, ""
}

func (eg *EquityGuard) computeStatsLocked(stats *EquityStats) (avg float64, wins, losses int) {
	if stats == nil || len(stats.Trades) == 0 {
		return 0, 0, 0
	}
	sum := 0.0
	for _, t := range stats.Trades {
		sum += t.PnL
		if t.PnL > 0 {
			wins++
		} else if t.PnL < 0 {
			losses++
		}
	}
	avg = sum / float64(len(stats.Trades))
	return avg, wins, losses
}
