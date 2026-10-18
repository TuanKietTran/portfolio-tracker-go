package domain

import (
	"errors"
	"github.com/shopspring/decimal"
)

const Split EventKind = "SPLIT"

// CorporateAction amounts are total portfolio cash amounts, not per-share rates.
// Split quantities must be exactly representable; no cash-in-lieu is generated.
type CorporateAction struct {
	Kind        EventKind       `json:"kind"`
	Currency    string          `json:"currency"`
	Amount      decimal.Decimal `json:"amount,omitempty"`
	Numerator   int64           `json:"numerator,omitempty"`
	Denominator int64           `json:"denominator,omitempty"`
}

func (a CorporateAction) Validate() error {
	if err := ValidateCashAmount(decimal.Zero, a.Currency); err != nil {
		return err
	}
	if err := ValidateDecimalPrecision(a.Amount); err != nil {
		return err
	}
	switch a.Kind {
	case Split:
		if !a.Amount.IsZero() || a.Numerator <= 0 || a.Denominator <= 0 {
			return errors.New("split requires positive numerator/denominator and no cash amount")
		}
	case Dividend, Fee:
		if a.Numerator != 0 || a.Denominator != 0 || !a.Amount.IsPositive() {
			return errors.New("cash action requires positive total amount and no split ratio")
		}
		if err := ValidateCashAmount(a.Amount, a.Currency); err != nil {
			return err
		}
	default:
		return errors.New("corporate kind must be SPLIT, DIVIDEND or FEE")
	}
	return nil
}

// ApplyCorporateAction returns independent holdings and the signed cash effect.
// Fees may be charged on a previously held, now closed position. Dividends and
// splits require current shares. The caller must atomically fund any fee debit.
func ApplyCorporateAction(position AccountPosition, a CorporateAction) (AccountPosition, decimal.Decimal, error) {
	if err := a.Validate(); err != nil {
		return position, decimal.Zero, err
	}
	if position.Currency != a.Currency {
		return position, decimal.Zero, errors.New("position currency mismatch")
	}
	if a.Kind != Fee && !position.Quantity.IsPositive() {
		return position, decimal.Zero, errors.New("corporate action requires held shares")
	}
	if a.Kind == Split {
		updated, err := ApplySplit(position, a.Numerator, a.Denominator)
		return updated, decimal.Zero, err
	}
	updated, err := ApplyEvent(position, AccountingEvent{Kind: a.Kind, Currency: a.Currency, Amount: a.Amount}, position.Method)
	if err != nil {
		return position, decimal.Zero, err
	}
	delta := a.Amount
	if a.Kind == Fee {
		delta = delta.Neg()
	}
	return updated, delta, nil
}
