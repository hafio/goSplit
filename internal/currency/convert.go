// Package currency provides exchange-rate lookup (pluggable providers + DB
// cache) and exact minor-unit conversion. Money is never a float: conversion is
// done with math/big rationals and rounded to the target currency's minor unit.
package currency

import (
	"fmt"
	"math/big"
	"strings"
)

// decimals returns a currency's minor-unit digit count (mirrors money.decimals).
func decimals(cur string) int {
	switch strings.ToUpper(cur) {
	case "JPY", "KRW", "VND", "CLP", "ISK", "HUF", "TWD", "UGX", "XAF", "XOF":
		return 0
	case "BHD", "KWD", "OMR", "TND", "IQD", "JOD", "LYD":
		return 3
	default:
		return 2
	}
}

// Convert converts amountFrom (minor units of fromCur) into minor units of
// toCur at the given decimal rate string (e.g. "0.9012" meaning 1 fromCur =
// 0.9012 toCur). It accounts for differing minor-unit scales and rounds half
// away from zero. rate must be a positive decimal.
func Convert(amountFrom int64, fromCur, toCur, rate string) (int64, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(rate))
	if !ok {
		return 0, fmt.Errorf("currency: invalid rate %q", rate)
	}
	if r.Sign() <= 0 {
		return 0, fmt.Errorf("currency: rate must be positive, got %q", rate)
	}
	// value_to_minor = amountFrom_minor * rate * 10^(toDec) / 10^(fromDec)
	res := new(big.Rat).SetInt64(amountFrom)
	res.Mul(res, r)

	fromDec, toDec := decimals(fromCur), decimals(toCur)
	if toDec > fromDec {
		res.Mul(res, ratPow10(toDec-fromDec))
	} else if fromDec > toDec {
		res.Quo(res, ratPow10(fromDec-toDec))
	}
	return roundRat(res), nil
}

// ratPow10 returns 10^n as a *big.Rat (n >= 0).
func ratPow10(n int) *big.Rat {
	i := big.NewInt(1)
	ten := big.NewInt(10)
	for k := 0; k < n; k++ {
		i.Mul(i, ten)
	}
	return new(big.Rat).SetInt(i)
}

// roundRat rounds a rational to the nearest int64, half away from zero.
func roundRat(r *big.Rat) int64 {
	num := new(big.Int).Set(r.Num())
	den := new(big.Int).Set(r.Denom())
	neg := num.Sign() < 0
	if neg {
		num.Neg(num)
	}
	// floor division
	q := new(big.Int)
	rem := new(big.Int)
	q.QuoRem(num, den, rem)
	// round half away from zero: if 2*rem >= den, bump.
	twice := new(big.Int).Lsh(rem, 1)
	if twice.Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if neg {
		q.Neg(q)
	}
	return q.Int64()
}
