package sandbox

import "github.com/shopspring/decimal"

func mustDecimal(v string) decimal.Decimal { return decimal.RequireFromString(v) }
