package command

import (
	"context"
	"errors"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrReturnsUnavailable = errors.New("returns require a snapshot-capable price feed")

const SecondsPerYear int64 = 365 * 24 * 60 * 60

type ReturnOptions struct {
	Method string
	MaxAge int64
	IRR    domain.IRROptions
}
type ReturnSolverReport struct {
	Method           string          `json:"method"`
	Iterations       int             `json:"iterations"`
	MaxIterations    int             `json:"max_iterations"`
	RateTolerance    decimal.Decimal `json:"rate_tolerance"`
	NPVTolerance     decimal.Decimal `json:"npv_tolerance"`
	NPVResidual      decimal.Decimal `json:"npv_residual"`
	RateBracketWidth decimal.Decimal `json:"rate_bracket_width"`
}
type PortfolioReturn struct {
	Currency      string              `json:"currency"`
	From          time.Time           `json:"from"`
	To            time.Time           `json:"to"`
	Method        string              `json:"method"`
	Rate          decimal.Decimal     `json:"rate"`
	Annualized    bool                `json:"annualized"`
	YearBasis     string              `json:"year_basis,omitempty"`
	StartNAV      decimal.Decimal     `json:"start_nav"`
	EndNAV        decimal.Decimal     `json:"end_nav"`
	ExternalFlows int                 `json:"external_flows"`
	Subperiods    int                 `json:"subperiods,omitempty"`
	Solver        *ReturnSolverReport `json:"solver,omitempty"`
}

func DefaultIRROptions() domain.IRROptions {
	return domain.IRROptions{RateTolerance: decimal.RequireFromString("0.0000000001"), NPVTolerance: decimal.RequireFromString("0.00000001"), MaxIterations: 128}
}

// Returns freezes a holdings/flow window and a price view separately. Mark tuples
// are immutable; all valuations in this calculation use the same copied price set.
// Investor flows are selected from (from,to]; starting NAV substitutes starting
// capital exactly once. Per-flow snapshots preserve same-time command ordering.
func (s *PortfolioService) Returns(ctx context.Context, id uuid.UUID, currency string, from, to time.Time, options ReturnOptions) (PortfolioReturn, error) {
	from, to = from.UTC(), to.UTC()
	if err := domain.ValidateCashAmount(decimal.Zero, currency); err != nil {
		return PortfolioReturn{}, err
	}
	if from.Unix() <= 0 || to.Year() > 9999 || !to.After(from) || to.Sub(from) > time.Duration(10*SecondsPerYear)*time.Second || options.MaxAge < 0 {
		return PortfolioReturn{}, errors.New("return period must increase within3650 days with nonnegative max_age")
	}
	if options.Method != "TWR" && options.Method != "MWR" {
		return PortfolioReturn{}, errors.New("method must be TWR or MWR")
	}
	if options.Method == "MWR" {
		if err := options.IRR.Validate(); err != nil {
			return PortfolioReturn{}, err
		}
	}
	window, err := s.state.ReturnWindow(ctx, id, from, to)
	if err != nil {
		return PortfolioReturn{}, err
	}
	snapshotter, ok := s.prices.(repository.PriceSnapshotter)
	if !ok {
		return PortfolioReturn{}, ErrReturnsUnavailable
	}
	frozen := *s
	frozen.prices = snapshotter.SnapshotPrices()
	if frozen.prices == nil {
		return PortfolioReturn{}, ErrReturnsUnavailable
	}
	start, err := frozen.valueSnapshot(ctx, window.Start, currency, from, options.MaxAge)
	if err != nil {
		return PortfolioReturn{}, err
	}
	end, err := frozen.valueSnapshot(ctx, window.End, currency, to, options.MaxAge)
	if err != nil {
		return PortfolioReturn{}, err
	}
	if !start.NetAssetValue.IsPositive() {
		return PortfolioReturn{}, errors.New("positive starting NAV required")
	}
	result := PortfolioReturn{Currency: currency, From: from, To: to, Method: options.Method, StartNAV: start.NetAssetValue, EndNAV: end.NetAssetValue, ExternalFlows: len(window.Flows)}
	periods := []decimal.Decimal{}
	base := start.NetAssetValue
	flows := []domain.CashFlow{{Year: 0, Amount: canonicalReturnAmount(start.NetAssetValue.Neg())}}
	var previous uint64
	for _, flow := range window.Flows {
		if flow.Currency != currency {
			return PortfolioReturn{}, errors.New("cross-currency investor flows require unsupported FX")
		}
		if flow.Sequence <= previous || !flow.OccurredAt.After(from) || flow.OccurredAt.After(to) {
			return PortfolioReturn{}, errors.New("invalid return-window flow ordering")
		}
		previous = flow.Sequence
		if flow.InvestorFlow.IsZero() || !flow.InvestorFlow.Equal(flow.CashDelta.Neg()) || !(flow.Kind == "FUND" && flow.CashDelta.IsPositive() || flow.Kind == "WITHDRAW" && flow.CashDelta.IsNegative()) {
			return PortfolioReturn{}, errors.New("invalid external investor-flow classification")
		}
		after, err := frozen.valueSnapshot(ctx, flow.Snapshot, currency, flow.OccurredAt, options.MaxAge)
		if err != nil {
			return PortfolioReturn{}, err
		}
		before := after.NetAssetValue.Sub(flow.CashDelta)
		if options.Method == "TWR" {
			if !before.IsPositive() || !after.NetAssetValue.IsPositive() {
				return PortfolioReturn{}, errors.New("TWR requires positive capital before/after every external flow")
			}
			periods = append(periods, before.DivRound(base, 18).Sub(decimal.NewFromInt(1)))
			base = after.NetAssetValue
		}
		flows = append(flows, domain.CashFlow{Year: flow.OccurredAt.Sub(from).Seconds() / float64(SecondsPerYear), Amount: canonicalReturnAmount(flow.InvestorFlow)})
	}
	if options.Method == "TWR" {
		if !end.NetAssetValue.IsPositive() {
			return PortfolioReturn{}, errors.New("TWR total loss or zero ending capital is unsupported")
		}
		periods = append(periods, end.NetAssetValue.DivRound(base, 18).Sub(decimal.NewFromInt(1)))
		rate, err := domain.TimeWeightedReturn(periods)
		if err != nil {
			return PortfolioReturn{}, err
		}
		result.Rate = rate.Round(18)
		result.Subperiods = len(periods)
		return result, nil
	}
	flows = append(flows, domain.CashFlow{Year: to.Sub(from).Seconds() / float64(SecondsPerYear), Amount: canonicalReturnAmount(end.NetAssetValue)})
	solution, err := domain.SolveConventionalIRR(flows, options.IRR)
	if err != nil {
		return PortfolioReturn{}, err
	}
	result.Rate = solution.Rate
	result.Annualized = true
	result.YearBasis = "ACT/365 fixed seconds"
	result.Solver = &ReturnSolverReport{Method: "bounded bisection; float64 discount factors; decimal cash", Iterations: solution.Iterations, MaxIterations: options.IRR.MaxIterations, RateTolerance: options.IRR.RateTolerance, NPVTolerance: options.IRR.NPVTolerance, NPVResidual: solution.Residual, RateBracketWidth: solution.BracketWidth}
	return result, nil
}

// Solver cash projections round half away from zero to18 fractional places;
// exact NAV remains separately reported. Canonicalization removes trailing zeros.
func canonicalReturnAmount(amount decimal.Decimal) decimal.Decimal {
	return decimal.RequireFromString(amount.Round(18).String())
}
