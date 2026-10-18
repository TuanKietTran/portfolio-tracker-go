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
	Symbol   string `json:"symbol"`
	Currency string `json:"currency"`
}

func (i Instrument) Validate() error {
	if len(i.Symbol) == 0 || len(i.Symbol) > 32 {
		return errors.New("symbol must contain 1 through 32 ASCII letters, digits, dot, underscore or hyphen")
	}
	for _, c := range i.Symbol {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return errors.New("invalid instrument symbol")
		}
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
	Quantity  decimal.Decimal
	UnitCost  decimal.Decimal // Display value; never the authoritative allocation source.
	CostBasis decimal.Decimal
}

type AccountPosition struct {
	Currency    string
	Method      CostMethod
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
	Currency  string
	Kind      EventKind
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal
	Amount    decimal.Decimal
}

// Clone returns an independent position snapshot, including its mutable lot slice.
func (p AccountPosition) Clone() AccountPosition {
	p.Lots = append([]Lot(nil), p.Lots...)
	return p
}

// ApplyEvent returns a new position, leaving the input unchanged on errors. Fees/dividends
// are cash income/expense and do not alter security quantity or cost basis.
func ApplyEvent(position AccountPosition, event AccountingEvent, method CostMethod) (AccountPosition, error) {
	position = position.Clone()
	if event.Currency != "" {
		money, err := NewMoney(decimal.Zero, event.Currency)
		if err != nil {
			return position, err
		}
		if position.Currency != "" && position.Currency != money.Currency {
			return position, errors.New("position currency mismatch")
		}
		position.Currency = money.Currency
	}
	if method != FIFO && method != AverageCost {
		return position, errors.New("unsupported cost method")
	}
	if position.Method != "" && position.Method != method {
		return position, errors.New("position cost method is immutable")
	}
	position.Method = method
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
	if err := ValidateDecimalPrecision(event.Quantity); err != nil {
		return position, err
	}
	if err := ValidateDecimalPrecision(event.UnitPrice); err != nil {
		return position, err
	}
	if !event.Quantity.IsPositive() || event.UnitPrice.IsNegative() {
		return position, errors.New("quantity must be positive and price non-negative")
	}
	if event.Kind == Buy {
		position.Quantity = position.Quantity.Add(event.Quantity)
		position.CostBasis = position.CostBasis.Add(event.Quantity.Mul(event.UnitPrice))
		if method == FIFO {
			position.Lots = append(position.Lots, Lot{Quantity: event.Quantity, UnitCost: event.UnitPrice, CostBasis: event.Quantity.Mul(event.UnitPrice)})
		}
		return position, nil
	}
	if event.Quantity.GreaterThan(position.Quantity) {
		return position, errors.New("oversell exceeds available quantity")
	}
	if method == AverageCost {
		removed := allocateBasis(position.CostBasis, event.Quantity, position.Quantity)
		position.RealizedPnL = position.RealizedPnL.Add(event.UnitPrice.Mul(event.Quantity).Sub(removed))
		position.CostBasis = position.CostBasis.Sub(removed)
	} else {
		remaining := event.Quantity
		for len(position.Lots) > 0 && remaining.IsPositive() {
			lot := &position.Lots[0]
			used := decimal.Min(lot.Quantity, remaining)
			basis := lot.CostBasis
			// Compatibility for pure-domain fixtures created before authoritative lot basis.
			if basis.IsZero() && lot.UnitCost.IsPositive() {
				basis = lot.Quantity.Mul(lot.UnitCost)
			}
			removed := allocateBasis(basis, used, lot.Quantity)
			position.RealizedPnL = position.RealizedPnL.Add(event.UnitPrice.Mul(used).Sub(removed))
			position.CostBasis = position.CostBasis.Sub(removed)
			lot.CostBasis = basis.Sub(removed)
			lot.Quantity = lot.Quantity.Sub(used)
			if lot.Quantity.IsPositive() {
				lot.UnitCost = lot.CostBasis.DivRound(lot.Quantity, 18)
			}
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

// allocateBasis rounds partial allocations half away from zero at 18 fractional
// places. The residual remains authoritative and is consumed exactly on full sale.
func allocateBasis(basis, used, available decimal.Decimal) decimal.Decimal {
	if used.Equal(available) {
		return basis
	}
	return basis.Mul(used).DivRound(available, 18)
}

// ApplySplit preserves exact total basis and rejects unrepresentable share quantities.
func ApplySplit(position AccountPosition, numerator, denominator int64) (AccountPosition, error) {
	position = position.Clone()
	if numerator <= 0 || denominator <= 0 {
		return position, errors.New("split ratio terms must be positive")
	}
	n, d := decimal.NewFromInt(numerator), decimal.NewFromInt(denominator)
	scaleQuantity := func(q decimal.Decimal) (decimal.Decimal, error) {
		numerator := q.Mul(n)
		result := numerator.DivRound(d, 18)
		if !result.Mul(d).Equal(numerator) {
			return decimal.Zero, errors.New("split share quantity is not exact at 18 decimal places; cash-in-lieu unsupported")
		}
		return result, nil
	}
	quantity, err := scaleQuantity(position.Quantity)
	if err != nil {
		return position, err
	}
	for i := range position.Lots {
		lot := &position.Lots[i]
		quantity, err := scaleQuantity(lot.Quantity)
		if err != nil {
			return AccountPosition{}, err
		}
		if lot.CostBasis.IsZero() && lot.UnitCost.IsPositive() {
			lot.CostBasis = lot.Quantity.Mul(lot.UnitCost)
		}
		lot.Quantity = quantity
		if quantity.IsPositive() {
			lot.UnitCost = lot.CostBasis.DivRound(quantity, 18)
		}
	}
	position.Quantity = quantity
	return position, nil
}
