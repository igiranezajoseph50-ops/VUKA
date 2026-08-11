import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatusBadge } from '../src/components/StatusBadge'

describe('StatusBadge', () => {
  it('renders the status text', () => {
    render(<StatusBadge status="SUCCESS" />)
    expect(screen.getByText('SUCCESS')).toBeTruthy()
  })

  it('applies the success emerald styling', () => {
    const { container } = render(<StatusBadge status="SUCCESS" />)
    const el = container.querySelector('span')
    expect(el?.className).toContain('emerald')
  })

  it('applies reversal styling', () => {
    const { container } = render(<StatusBadge status="REVERSED" />)
    const el = container.querySelector('span')
    expect(el?.className).toContain('slate')
  })

  it('falls back to pending styling for an unknown status', () => {
    const { container } = render(<StatusBadge status={'UNKNOWN' as never} />)
    const el = container.querySelector('span')
    expect(el?.className).toContain('amber')
  })
})