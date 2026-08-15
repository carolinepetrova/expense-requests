import type { RequestSummary, RequestView, Values } from './types'

/** THRESHOLD_CENTS mirrors the server's $1,000 boundary. */
export const THRESHOLD_CENTS = 100_000

/**
 * The conditional form rules, duplicated from the server on purpose.
 *
 * This copy decides what to *show* and what to mark as required while typing.
 * It is not what makes the rules true — the server validates on submit and its
 * fieldErrors always win. A UI that gets this wrong produces a 422, not a bad
 * request slipping through.
 */
export function shows(values: Values, field: keyof Values): boolean {
  switch (field) {
    case 'client':
      return values.billable
    case 'additionalJustification':
      return values.amountCents >= THRESHOLD_CENTS
    case 'otherReason':
      return values.expenseType === 'Other'
    default:
      return true
  }
}

/** why explains a conditional field, for the hint under it. */
export function why(field: keyof Values): string {
  switch (field) {
    case 'client':
      return 'Required because this expense is billable to a client.'
    case 'additionalJustification':
      return 'Required because the amount is $1,000 or more.'
    case 'otherReason':
      return 'Required because the expense type is Other.'
    default:
      return ''
  }
}

/**
 * What the current user may do, worked out locally so buttons can be hidden.
 *
 * The server enforces all of this independently; these are affordances, not
 * permissions.
 */
export function canEdit(r: RequestView, me: string): boolean {
  return r.requesterId === me && r.status === 'Draft'
}

export function canAct(r: RequestView | RequestSummary, me: string): boolean {
  return r.status === 'Submitted' && r.approverId === me
}

// ---------------------------------------------------------------- formatting

export function toCents(dollars: string): number {
  const n = Number.parseFloat(dollars)
  return Number.isFinite(n) ? Math.round(n * 100) : 0
}

export function fromCents(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function money(cents: number): string {
  return `$${fromCents(cents)}`
}

export function when(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
}
