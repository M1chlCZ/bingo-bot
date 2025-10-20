package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type StochasticOscillator struct {
	Overbought int `json:"overbought"`
	Oversold   int `json:"oversold"`
	Period     int `json:"period"`
	DPeriod    int `json:"dPeriod"`
}

func (so *StochasticOscillator) Calculate(candles []models.CandleStick) (k, d float64, err error) {
	p := so.Period
	dp := so.DPeriod
	if dp == 0 {
		dp = 3 // default 3 if not set
	}
	if p < 2 {
		return 0, 0, fmt.Errorf("stochastic: period must be >= 2 (got %d)", p)
	}
	if dp < 1 {
		return 0, 0, fmt.Errorf("stochastic: dPeriod must be >= 1 (got %d)", dp)
	}

	need := p + dp - 1
	if len(candles) < need {
		return 0, 0, fmt.Errorf("stochastic: not enough data (need >= %d candles, got %d)", need, len(candles))
	}

	start := len(candles) - need
	end := len(candles) - 1 // inclusive

	type deque struct{ idx []int }

	pushMax := func(dq *deque, arr []float64, i int) {
		for len(dq.idx) > 0 && arr[dq.idx[len(dq.idx)-1]] <= arr[i] {
			dq.idx = dq.idx[:len(dq.idx)-1]
		}
		dq.idx = append(dq.idx, i)
	}
	pushMin := func(dq *deque, arr []float64, i int) {
		for len(dq.idx) > 0 && arr[dq.idx[len(dq.idx)-1]] >= arr[i] {
			dq.idx = dq.idx[:len(dq.idx)-1]
		}
		dq.idx = append(dq.idx, i)
	}
	popOut := func(dq *deque, left int) {

		for len(dq.idx) > 0 && dq.idx[0] < left {
			dq.idx = dq.idx[1:]
		}
	}

	n := len(candles)
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		highs[i] = candles[i].High
		lows[i] = candles[i].Low
		closes[i] = candles[i].Close
	}

	var maxDQ, minDQ deque
	kVals := make([]float64, 0, dp)

	firstEnd := start + p - 1
	for i := start; i <= firstEnd; i++ {
		pushMax(&maxDQ, highs, i)
		pushMin(&minDQ, lows, i)
	}

	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}

	for i := firstEnd; i <= end; i++ {
		left := i - p + 1
		popOut(&maxDQ, left)
		popOut(&minDQ, left)

		if i > firstEnd {
			pushMax(&maxDQ, highs, i)
			pushMin(&minDQ, lows, i)
		}

		highest := highs[maxDQ.idx[0]]
		lowest := lows[minDQ.idx[0]]

		var kNow float64
		den := highest - lowest
		if math.Abs(den) < 1e-12 {
			kNow = 50.0 // flat range → neutral
		} else {
			kNow = (closes[i] - lowest) / den * 100.0
		}
		kVals = append(kVals, clamp01(kNow))
	}

	if len(kVals) < dp {

		last := 50.0
		if len(kVals) > 0 {
			last = kVals[len(kVals)-1]
		}
		return last, last, nil
	}

	k = kVals[len(kVals)-1]

	sum := 0.0
	for i := len(kVals) - dp; i < len(kVals); i++ {
		sum += kVals[i]
	}
	d = clamp01(sum / float64(dp))

	return k, d, nil
}
