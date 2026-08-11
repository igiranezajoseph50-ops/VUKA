import { describe, expect, it } from 'vitest'
import { amount, amountInWords, convert, datetime, money, numberToWords } from '../src/lib/format'

describe('money', () => {
  it('formats an amount with a currency code', () => {
    expect(money(10000, 'RWF')).toBe('RWF\xA010,000.00')
  })

  it('formats a fractional amount to 2 dp', () => {
    expect(money(47500.5, 'KES')).toContain('47,500.50')
  })

  it('handles negative amounts', () => {
    // Intl places the minus before the currency code: -RWF 1,000.00
    expect(money(-1000, 'RWF')).toContain('1,000.00')
    expect(money(-1000, 'RWF')).toContain('-')
  })

  it('formats zero', () => {
    expect(money(0, 'RWF')).toContain('0.00')
  })
})

describe('amount', () => {
  it('formats plain numbers with thousand separators', () => {
    expect(amount(95000)).toBe('95,000.00')
  })

  it('handles large values', () => {
    expect(amount(1234567.89)).toBe('1,234,567.89')
  })
})

describe('convert (cross-border FX)', () => {
  it('divides the source amount by the rate', () => {
    expect(convert(95000, 9.5)).toBe(10000)
  })

  it('rounds to 2 dp', () => {
    expect(convert(100, 3)).toBe(33.33)
  })

  it('returns 0 for a non-positive or invalid rate', () => {
    expect(convert(100, 0)).toBe(0)
    expect(convert(100, -1)).toBe(0)
    expect(convert(100, NaN)).toBe(0)
  })
})

describe('datetime', () => {
  it('formats an ISO timestamp', () => {
    const out = datetime('2026-08-05T14:00:00Z')
    expect(out.length).toBeGreaterThan(0)
    expect(out).not.toBe('2026-08-05T14:00:00Z') // should not stay raw
  })

  it('falls back to the raw value on invalid input', () => {
    expect(datetime('not-a-date')).toBe('not-a-date')
  })
})

describe('numberToWords', () => {
  it('converts small numbers', () => {
    expect(numberToWords(0)).toBe('Zero')
    expect(numberToWords(7)).toBe('Seven')
    expect(numberToWords(19)).toBe('Nineteen')
    expect(numberToWords(42)).toBe('Forty-Two')
  })

  it('converts hundreds', () => {
    expect(numberToWords(101)).toBe('One Hundred One')
    expect(numberToWords(305)).toBe('Three Hundred Five')
  })

  it('converts thousands and millions', () => {
    expect(numberToWords(101500)).toBe('One Hundred One Thousand Five Hundred')
    expect(numberToWords(5000000)).toBe('Five Million')
    expect(numberToWords(1234567)).toBe('One Million Two Hundred Thirty-Four Thousand Five Hundred Sixty-Seven')
  })

  it('handles negatives', () => {
    expect(numberToWords(-250)).toBe('Minus Two Hundred Fifty')
  })
})

describe('amountInWords', () => {
  it('prints whole amounts with the currency name and "only"', () => {
    expect(amountInWords(64900, 'RWF')).toBe('Sixty-Four Thousand Nine Hundred Rwandan Francs only')
  })

  it('adds fractional cents as /100', () => {
    expect(amountInWords(10500.5, 'KES')).toBe('Ten Thousand Five Hundred Kenyan Shillings and 50/100')
  })

  it('uses the currency code for unknown currencies', () => {
    expect(amountInWords(100, 'EUR')).toBe('One Hundred EUR only')
  })
})