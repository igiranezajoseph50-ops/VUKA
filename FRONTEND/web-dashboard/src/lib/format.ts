// Money + data formatting helpers (D10): all amounts render with
// Intl.NumberFormat + the ISO currency code. No floating-point strings.

const compact = new Intl.NumberFormat('en', {
  notation: 'compact',
  maximumFractionDigits: 1,
})

/** Format an amount as currency, e.g. 10000 RWF -> "RWF 10,000.00". */
export function money(amount: number, currency = 'RWF'): string {
  return new Intl.NumberFormat('en-UG', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount)
}

/** Compact money for KPIs, e.g. 1,450,000 RWF -> "1.5M". */
export function moneyCompact(amount: number, currency = 'RWF'): string {
  const ccy = currency === 'RWF' ? 'RWF' : currency === 'KES' ? 'KSh' : currency
  if (Math.abs(amount) >= 1000) {
    return `${ccy} ${compact.format(amount)}`
  }
  return `${ccy} ${amount.toFixed(0)}`
}

/** Plain two-decimal number, no currency symbol. */
export function amount(amount: number): string {
  return new Intl.NumberFormat('en-UG', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount)
}

/** Compact local date + time. */
export function datetime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('en-GB', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** Absolute date only, e.g. "05 Aug 2026". */
export function dateOnly(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })
}

/** Convert source amount across FX: amount / fx_rate, 2 dp. */
export function convert(sourceAmount: number, fxRate: number): number {
  if (!fxRate || fxRate <= 0 || Number.isNaN(fxRate)) return 0
  return Math.round((sourceAmount / fxRate) * 100) / 100
}

/** Generate a stock ticker-style preview of a balance trend (prototype). */
export function trendLabel(current: number, prev: number): string {
  if (prev === 0) return '—'
  const pct = ((current - prev) / Math.abs(prev)) * 100
  if (pct >= 0) return `▲ ${pct.toFixed(1)}%`
  return `▼ ${Math.abs(pct).toFixed(1)}%`
}

const WORDS_ONES = [
  'Zero', 'One', 'Two', 'Three', 'Four', 'Five', 'Six', 'Seven', 'Eight', 'Nine',
  'Ten', 'Eleven', 'Twelve', 'Thirteen', 'Fourteen', 'Fifteen', 'Sixteen',
  'Seventeen', 'Eighteen', 'Nineteen',
]
const WORDS_TENS = ['', '', 'Twenty', 'Thirty', 'Forty', 'Fifty', 'Sixty', 'Seventy', 'Eighty', 'Ninety']
const WORDS_SCALES = ['', 'Thousand', 'Million', 'Billion', 'Trillion']

function threeDigitsWords(n: number): string {
  const out: string[] = []
  const hundreds = Math.floor(n / 100)
  const rest = n % 100
  if (hundreds > 0) out.push(`${WORDS_ONES[hundreds]} Hundred`)
  if (rest > 0) {
    if (rest < 20) out.push(WORDS_ONES[rest])
    else {
      const tens = Math.floor(rest / 10)
      const ones = rest % 10
      if (ones > 0) out.push(`${WORDS_TENS[tens]}-${WORDS_ONES[ones]}`)
      else out.push(WORDS_TENS[tens])
    }
  }
  return out.join(' ')
}

/** Whole-number integer to English words, e.g. 101500 -> "One Hundred One Thousand Five Hundred". */
export function numberToWords(n: number): string {
  if (!Number.isFinite(n)) return ''
  if (n < 0) return `Minus ${numberToWords(-n)}`
  if (n === 0) return WORDS_ONES[0]
  let remaining = Math.floor(n)
  const groups: number[] = []
  while (remaining > 0) {
    groups.push(remaining % 1000)
    remaining = Math.floor(remaining / 1000)
  }
  const parts: string[] = []
  for (let i = groups.length - 1; i >= 0; i--) {
    if (groups[i] === 0) continue
    const words = threeDigitsWords(groups[i])
    parts.push(i > 0 ? `${words} ${WORDS_SCALES[i]}` : words)
  }
  return parts.join(' ')
}

/** Amount in words for a printed invoice, e.g. 64900 RWF -> "Sixty-Four Thousand Nine Hundred Rwandan Francs only". */
export function amountInWords(amountValue: number, currency = 'RWF'): string {
  const currencyName = currency === 'RWF' ? 'Rwandan Francs' : currency === 'KES' ? 'Kenyan Shillings' : currency
  const raw = Math.round(Math.abs(amountValue) * 100) / 100
  const whole = Math.floor(raw)
  const cents = Math.round((raw - whole) * 100)
  const words = numberToWords(whole)
  if (cents > 0) {
    return `${words} ${currencyName} and ${cents}/100`
  }
  return `${words} ${currencyName} only`
}