package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Money is an exact decimal amount paired with its currency. No conversion is implicit.
type Money struct {
	Amount   decimal.Decimal
	Currency string
}

func NewMoney(amount decimal.Decimal, currency string) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 || currency[0] < 'A' || currency[0] > 'Z' || currency[1] < 'A' || currency[1] > 'Z' || currency[2] < 'A' || currency[2] > 'Z' {
		return Money{}, errors.New("currency must be a three-letter code")
	}
	return Money{Amount: amount, Currency: currency}, nil
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("currency mismatch")
	}
	return Money{Amount: m.Amount.Add(other.Amount), Currency: m.Currency}, nil
}

type Instrument struct {
	Symbol   string
	Currency string
}

func (i Instrument) Validate() error {
	if strings.TrimSpace(i.Symbol) == "" {
		return errors.New("symbol is required")
	}
	_, err := NewMoney(decimal.Zero, i.Currency)
	return err
}

type CostMethod string

const (
	FIFO        CostMethod = "FIFO"
	AverageCost CostMethod = "AVERAGE_COST"
)

type Lot struct {
	Quantity decimal.Decimal
	UnitCost decimal.Decimal
}

type AccountPosition struct {
	Quantity    decimal.Decimal
	CostBasis   decimal.Decimal
	RealizedPnL decimal.Decimal
	Lots        []Lot
}

type EventKind string

const (
	Buy      EventKind = "BUY"
	Sell     EventKind = "SELL"
	Dividend EventKind = "DIVIDEND"
	Fee      EventKind = "FEE"
)

type AccountingEvent struct {
	Kind      EventKind
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal
	Amount    decimal.Decimal
}

// ApplyEvent returns a new position, leaving the input unchanged on errors. Fees/dividends
// are cash income/expense and do not alter security quantity or cost basis.
func ApplyEvent(position AccountPosition, event AccountingEvent, method CostMethod) (AccountPosition, error) {
	if method != FIFO && method != AverageCost {
		return position, errors.New("unsupported cost method")
	}
	if event.Kind == Dividend || event.Kind == Fee {
		if event.Amount.IsNegative() {
			return position, errors.New("cash event amount cannot be negative")
		}
		if event.Kind == Dividend {
			position.RealizedPnL = position.RealizedPnL.Add(event.Amount)
		} else {
			position.RealizedPnL = position.RealizedPnL.Sub(event.Amount)
		}
		return position, nil
	}
	if event.Kind != Buy && event.Kind != Sell {
		return position, errors.New("unsupported event kind")
	}
	if !event.Quantity.IsPositive() || event.UnitPrice.IsNegative() {
		return position, errors.New("quantity must be positive and price non-negative")
	}
	if event.Kind == Buy {
		position.Quantity = position.Quantity.Add(event.Quantity)
		position.CostBasis = position.CostBasis.Add(event.Quantity.Mul(event.UnitPrice))
		if method == FIFO {
			position.Lots = append(position.Lots, Lot{Quantity: event.Quantity, UnitCost: event.UnitPrice})
		}
		return position, nil
	}
	if event.Quantity.GreaterThan(position.Quantity) {
		return position, errors.New("oversell exceeds available quantity")
	}
	if method == AverageCost {
		unitCost := decimal.Zero
		if position.Quantity.IsPositive() {
			unitCost = position.CostBasis.Div(position.Quantity)
		}
		position.RealizedPnL = position.RealizedPnL.Add(event.UnitPrice.Sub(unitCost).Mul(event.Quantity))
		position.CostBasis = position.CostBasis.Sub(unitCost.Mul(event.Quantity))
	} else {
		remaining := event.Quantity
		for len(position.Lots) > 0 && remaining.IsPositive() {
			lot := &position.Lots[0]
			used := decimal.Min(lot.Quantity, remaining)
			position.RealizedPnL = position.RealizedPnL.Add(event.UnitPrice.Sub(lot.UnitCost).Mul(used))
			position.CostBasis = position.CostBasis.Sub(lot.UnitCost.Mul(used))
			lot.Quantity = lot.Quantity.Sub(used)
			remaining = remaining.Sub(used)
			if lot.Quantity.IsZero() {
				position.Lots = position.Lots[1:]
			}
		}
		if remaining.IsPositive() {
			return AccountPosition{}, fmt.Errorf("lot quantity invariant violated: %s", remaining)
		}
	}
	position.Quantity = position.Quantity.Sub(event.Quantity)
	if position.Quantity.IsZero() {
		position.CostBasis = decimal.Zero
		position.Lots = nil
	}
	return position, nil
}

// ApplySplit scales quantity and inversely scales per-unit lot cost; total cost basis is conserved.
func ApplySplit(position AccountPosition, numerator, denominator int64) (AccountPosition, error) {
	if numerator <= 0 || denominator <= 0 {
		return position, errors.New("split ratio terms must be positive")
	}
	ratio := decimal.NewFromInt(numerator).Div(decimal.NewFromInt(denominator))
	position.Quantity = position.Quantity.Mul(ratio)
	for i := range position.Lots {
		position.Lots[i].Quantity = position.Lots[i].Quantity.Mul(ratio)
		position.Lots[i].UnitCost = position.Lots[i].UnitCost.Div(ratio)
	}
	return position, nil
}
