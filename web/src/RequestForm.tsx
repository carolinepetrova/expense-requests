import { useState } from 'react'

import { ApiError, createRequest, submitRequest, updateRequest } from './api'
import { fromCents, shows, toCents, why } from './rules'
import { expenseTypes, type Client, type RequestView, type User, type Values } from './types'

interface Props {
  me: User
  clients: Client[]
  existing?: RequestView
  onDone: (id: string) => void
  onCancel: () => void
}

const blank: Values = {
  expenseType: '',
  amountCents: 0,
  description: '',
  billable: false,
  client: '',
  additionalJustification: '',
  otherReason: '',
}

export function RequestForm({ me, clients, existing, onDone, onCancel }: Props) {
  const [values, setValues] = useState<Values>(existing?.values ?? blank)
  const [amount, setAmount] = useState(
    existing ? fromCents(existing.values.amountCents) : '',
  )
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState('')

  const set = <K extends keyof Values>(key: K, value: Values[K]) =>
    setValues((v) => ({ ...v, [key]: value }))

  // Saving and submitting are separate on purpose: a draft may be incomplete,
  // and the rules only bite on submit.
  async function save(thenSubmit: boolean) {
    setError('')
    setFieldErrors({})

    try {
      const saved = existing
        ? await updateRequest(me.id, existing.id, values)
        : await createRequest(me.id, values)

      if (thenSubmit) await submitRequest(me.id, saved.id)
      onDone(saved.id)
    } catch (e) {
      if (e instanceof ApiError && e.status === 422) {
        setFieldErrors(e.byField())
        setError('Some fields need attention before this can be submitted.')
        return
      }
      setError(e instanceof Error ? e.message : 'Something went wrong.')
    }
  }

  const field = (name: keyof Values, label: string, input: React.ReactNode) => (
    <label className={fieldErrors[name] ? 'field invalid' : 'field'}>
      <span>{label}</span>
      {input}
      {why(name) && <small className="hint">{why(name)}</small>}
      {fieldErrors[name] && <small className="error">{fieldErrors[name]}</small>}
    </label>
  )

  return (
    <section className="form">
      <h2>{existing ? `Edit ${existing.id}` : 'New request'}</h2>

      {field(
        'expenseType',
        'Expense type',
        <select
          value={values.expenseType}
          onChange={(e) => set('expenseType', e.target.value as Values['expenseType'])}
        >
          <option value="">Choose…</option>
          {expenseTypes.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>,
      )}

      {field(
        'amountCents',
        'Amount',
        <input
          type="number"
          min="0"
          step="0.01"
          value={amount}
          onChange={(e) => {
            setAmount(e.target.value)
            set('amountCents', toCents(e.target.value))
          }}
        />,
      )}

      {field(
        'description',
        'Description',
        <input
          type="text"
          value={values.description}
          onChange={(e) => set('description', e.target.value)}
        />,
      )}

      <label className="field inline">
        <input
          type="checkbox"
          checked={values.billable}
          onChange={(e) => set('billable', e.target.checked)}
        />
        <span>Billable to a client</span>
      </label>

      {shows(values, 'client') &&
        field(
          'client',
          'Client',
          <select value={values.client ?? ''} onChange={(e) => set('client', e.target.value)}>
            <option value="">Choose…</option>
            {clients.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>,
        )}

      {shows(values, 'additionalJustification') &&
        field(
          'additionalJustification',
          'Additional justification',
          <textarea
            rows={3}
            value={values.additionalJustification ?? ''}
            onChange={(e) => set('additionalJustification', e.target.value)}
          />,
        )}

      {shows(values, 'otherReason') &&
        field(
          'otherReason',
          'What is this for?',
          <input
            type="text"
            value={values.otherReason ?? ''}
            onChange={(e) => set('otherReason', e.target.value)}
          />,
        )}

      {error && <p className="error">{error}</p>}

      <div className="actions">
        <button onClick={() => save(false)}>Save draft</button>
        <button className="primary" onClick={() => save(true)}>
          Save and submit
        </button>
        <button className="link" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </section>
  )
}
