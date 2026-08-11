import { describe, expect, it } from 'vitest'
import {
  invoiceSubtotal,
  invoiceVat,
  invoiceTotal,
  type Invoice,
} from '../src/components/InvoiceForm'

function makeInvoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    number: 'INV-001',
    issue_date: '2026-08-05',
    due_date: '2026-09-04',
    currency: 'RWF',
    counterparty: 'Kethan Gasana',
    counterparty_address: 'Nairobi',
    line_items: [
      { description: 'Coffee beans', quantity: 500, unitPrice: 100 },
      { description: 'Freight', quantity: 1, unitPrice: 5000 },
    ],
    vat_rate: 18,
    terms: 'Net 30',
    notes: '',
    created_at: '2026-08-05T00:00:00Z',
    ...overrides,
  }
}

describe('invoice math', () => {
  it('computes subtotal = sum(qty × unit price)', () => {
    const inv = makeInvoice()
    // 500×100 + 1×5000 = 55,000
    expect(invoiceSubtotal(inv)).toBe(55000)
  })

  it('computes VAT as a percentage of subtotal', () => {
    const inv = makeInvoice()
    // 55,000 × 18% = 9,900
    expect(invoiceVat(inv)).toBe(9900)
  })

  it('computes total including VAT', () => {
    const inv = makeInvoice()
    expect(invoiceTotal(inv)).toBe(64900)
  })

  it('handles an empty invoice', () => {
    const inv = makeInvoice({ line_items: [] })
    expect(invoiceSubtotal(inv)).toBe(0)
    expect(invoiceTotal(inv)).toBe(0)
  })

  it('supports a zero VAT rate', () => {
    const inv = makeInvoice({ vat_rate: 0 })
    expect(invoiceVat(inv)).toBe(0)
    expect(invoiceTotal(inv)).toBe(55000)
  })

  it('supports KES currency without changing amounts', () => {
    const inv = makeInvoice({ currency: 'KES' })
    expect(invoiceTotal(inv)).toBe(64900)
  })
})