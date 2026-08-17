package community

import (
	"fmt"
	"math/big"
	"strings"
)

// A bounded arithmetic grammar, not an expression language: no identifiers and
// no calls, so nothing here reaches anything. See docs/sirens-echo-tools.md.

// arithmeticParser walks the expression once, left to right.
type arithmeticParser struct {
	input []rune
	at    int
}

// evaluateArithmetic returns the exact value of an expression, or the reason it
// could not. Rational rather than floating point, so 0.1 + 0.2 is 0.3.
func evaluateArithmetic(expression string) (*big.Rat, error) {
	parser := &arithmeticParser{input: []rune(expression)}
	value, err := parser.sum()
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if parser.at < len(parser.input) {
		return nil, fmt.Errorf(
			"that is not arithmetic: %q is not an operator or a number",
			string(parser.input[parser.at]))
	}
	return value, nil
}

// sum is the lowest precedence, addition and subtraction.
func (p *arithmeticParser) sum() (*big.Rat, error) {
	value, err := p.product()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.accept('+'):
			right, err := p.product()
			if err != nil {
				return nil, err
			}
			value = new(big.Rat).Add(value, right)
		case p.accept('-'):
			right, err := p.product()
			if err != nil {
				return nil, err
			}
			value = new(big.Rat).Sub(value, right)
		default:
			return value, nil
		}
	}
}

// product is multiplication and division, which bind tighter than addition.
func (p *arithmeticParser) product() (*big.Rat, error) {
	value, err := p.power()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.accept('*'), p.accept('×'):
			right, err := p.power()
			if err != nil {
				return nil, err
			}
			value = new(big.Rat).Mul(value, right)
		case p.accept('/'), p.accept('÷'):
			right, err := p.power()
			if err != nil {
				return nil, err
			}
			if right.Sign() == 0 {
				return nil, fmt.Errorf("that divides by zero, which has no answer")
			}
			value = new(big.Rat).Quo(value, right)
		default:
			return value, nil
		}
	}
}

// power is right associative, so 2^3^2 is 2^9. The exponent is a whole number,
// because a fractional one is a root and not exact.
func (p *arithmeticParser) power() (*big.Rat, error) {
	base, err := p.unary()
	if err != nil {
		return nil, err
	}
	if !p.accept('^') {
		return base, nil
	}
	exponent, err := p.power()
	if err != nil {
		return nil, err
	}
	if !exponent.IsInt() {
		return nil, fmt.Errorf("a fractional exponent is a root, which this cannot do exactly")
	}
	whole := exponent.Num()
	limit := int64(maxCalculatorExponent)
	if !whole.IsInt64() || whole.Int64() > limit || whole.Int64() < -limit {
		return nil, fmt.Errorf(
			"an exponent beyond %d would produce a number too large to be useful",
			maxCalculatorExponent)
	}
	return raiseRational(base, whole.Int64())
}

// raiseRational raises an exact value to a whole power, negative included.
func raiseRational(base *big.Rat, exponent int64) (*big.Rat, error) {
	if exponent < 0 {
		if base.Sign() == 0 {
			return nil, fmt.Errorf("that divides by zero, which has no answer")
		}
		positive, err := raiseRational(base, -exponent)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Inv(positive), nil
	}
	result := new(big.Rat).SetInt64(1)
	for count := int64(0); count < exponent; count++ {
		result.Mul(result, base)
	}
	return result, nil
}

// unary is a leading sign, so -3 and --3 both read.
func (p *arithmeticParser) unary() (*big.Rat, error) {
	switch {
	case p.accept('-'):
		value, err := p.unary()
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Neg(value), nil
	case p.accept('+'):
		return p.unary()
	}
	return p.atom()
}

// atom is a number or a parenthesised expression, either optionally followed by
// a per cent sign.
func (p *arithmeticParser) atom() (*big.Rat, error) {
	p.skipSpace()
	if p.at >= len(p.input) {
		return nil, fmt.Errorf("that expression stops before it says what to calculate")
	}
	var value *big.Rat
	if p.accept('(') {
		inner, err := p.sum()
		if err != nil {
			return nil, err
		}
		if !p.accept(')') {
			return nil, fmt.Errorf("that expression opens a bracket it never closes")
		}
		value = inner
	} else {
		number, err := p.number()
		if err != nil {
			return nil, err
		}
		value = number
	}
	if p.accept('%') {
		value = new(big.Rat).Quo(value, new(big.Rat).SetInt64(100))
	}
	return value, nil
}

// number reads one decimal literal exactly. Separators are accepted because a
// member writing a price writes 1,250 rather than 1250.
func (p *arithmeticParser) number() (*big.Rat, error) {
	p.skipSpace()
	var digits strings.Builder
	seenDot := false
	for p.at < len(p.input) {
		current := p.input[p.at]
		switch {
		case current >= '0' && current <= '9':
			digits.WriteRune(current)
		case current == ',' || current == '_':
			// A thousands separator carries no value, so it is dropped rather
			// than ending the number and leaving a stray character behind.
		case current == '.' && !seenDot:
			seenDot = true
			digits.WriteRune(current)
		default:
			return p.parseDigits(digits.String())
		}
		p.at++
	}
	return p.parseDigits(digits.String())
}

func (p *arithmeticParser) parseDigits(digits string) (*big.Rat, error) {
	value, ok := new(big.Rat).SetString(digits)
	if !ok || digits == "" || digits == "." {
		if p.at < len(p.input) {
			return nil, fmt.Errorf(
				"that is not arithmetic: %q is not an operator or a number",
				string(p.input[p.at]))
		}
		return nil, fmt.Errorf("that expression stops before it says what to calculate")
	}
	if len(digits) > maxCalculatorDigits {
		return nil, fmt.Errorf(
			"a number longer than %d digits is beyond what this answers",
			maxCalculatorDigits)
	}
	return value, nil
}

// accept consumes one operator, skipping the space in front of it.
func (p *arithmeticParser) accept(want rune) bool {
	p.skipSpace()
	if p.at < len(p.input) && p.input[p.at] == want {
		p.at++
		return true
	}
	return false
}

func (p *arithmeticParser) skipSpace() {
	for p.at < len(p.input) &&
		(p.input[p.at] == ' ' || p.input[p.at] == '\t' || p.input[p.at] == '\n') {
		p.at++
	}
}
