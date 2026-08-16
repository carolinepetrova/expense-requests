// Mirrors what the API returns. Names are not on these types: the server sends
// identifiers, and the UI resolves them against the user list it already loads
// for the picker.

export type Status = 'Draft' | 'Submitted' | 'Approved' | 'Rejected'

export type ExpenseType = 'Travel' | 'Software' | 'Equipment' | 'Meal' | 'Other'

export const expenseTypes: ExpenseType[] = [
  'Travel',
  'Software',
  'Equipment',
  'Meal',
  'Other',
]

export interface User {
  id: string
  name: string
  role: 'employee' | 'manager' | 'finance'
  managerId: string | null
}

export interface Client {
  id: string
  name: string
}

export interface Values {
  expenseType: ExpenseType | ''
  amountCents: number
  description: string
  billable: boolean
  client?: string | null
  additionalJustification?: string | null
  otherReason?: string | null
}

export interface Step {
  name: string
  approverId: string
  status: 'Pending' | 'Approved' | 'Rejected'
  comment?: string
  actedAt?: string | null
}

/** One line of history. The server sends only these four fields — the write
 *  side's step indexes and compiled chain stay on the server. */
export interface TimelineEntry {
  type: 'created' | 'submitted' | 'stepApproved' | 'approved' | 'rejected'
  at: string
  actorId: string
  comment?: string
}

export interface RequestView {
  id: string
  requesterId: string
  status: Status
  approverId: string | null
  values: Values
  steps: Step[]
  timeline: TimelineEntry[]
  createdAt: string
  updatedAt: string
}

export interface RequestSummary {
  id: string
  requesterId: string
  status: Status
  approverId: string | null
  expenseType: ExpenseType
  amountCents: number
  description: string
  updatedAt: string
}

export interface FieldError {
  field: string
  code: string
  message: string
}
