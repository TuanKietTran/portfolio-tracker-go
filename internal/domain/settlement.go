package domain

import (
	"errors"
	"github.com/shopspring/decimal"
)

// ValidateDecimalPrecision bounds decimal work before arithmetic or replay comparisons.
func ValidateDecimalPrecision(amount decimal.Decimal) error {
	if amount.Exponent() < -18 || amount.Exponent() > 18 || amount.NumDigits() > 30 {
		return errors.New("decimal precision exceeds supported bounds")
	}
	return nil
}

// ValidateCashAmount uses the RejectFractionalMinorUnits policy: no implicit
// rounding or currency conversion occurs at the cash boundary. Quantities and
// prices may be fractional, but their trade notional must settle exactly.
// Only these synthetic settlement currencies are supported initially.
func ValidateCashAmount(amount decimal.Decimal, currency string) error {
	var scale int32
	switch currency {
	case "USD", "EUR", "GBP":
		scale = 2
	case "JPY":
		scale = 0
	default:
		return errors.New("unsupported settlement currency; use USD, EUR, GBP or JPY")
	}
	if err := ValidateDecimalPrecision(amount); err != nil {
		return err
	}
	if amount.IsNegative() {
		return errors.New("cash amount cannot be negative")
	}
	if !amount.Equal(amount.Truncate(scale)) {
		return errors.New("fractional minor units are not supported")
	}
	return nil
}
