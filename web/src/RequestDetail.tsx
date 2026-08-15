import { useCallback, useEffect, useState } from 'react'

import { approveRequest, getRequest, rejectRequest, submitRequest } from './api'
import { canAct, canEdit, money, when } from './rules'
import type { Event, RequestView, User } from './types'

interface Props {
  me: User
  names: Map<string, string>
  id: string
  onEdit: (id: string) => void
  onBack: () => void
}

const said: Record<Event['type'], string> = {
  created: 'created this request',
  submitted: 'submitted it for approval',
  stepApproved: 'approved their step',
  approved: 'approved it',
  rejected: 'rejected it',
}

export function RequestDetail({ me, names, id, onEdit, onBack }: Props) {
  const [r, setR] = useState<RequestView | null>(null)
  const [comment, setComment] = useState('')
  const [error, setError] = useState('')

  const load = useCallback(() => {
    getRequest(me.id, id)
      .then((v) => {
        setR(v)
        setError('')
      })
      .catch((e: Error) => setError(e.message))
  }, [me.id, id])

  useEffect(load, [load])

  async function act(fn: () => Promise<RequestView>) {
    try {
      setR(await fn())
      setComment('')
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Something went wrong.')
    }
  }

  if (!r) return <p className="error">{error || 'Loading…'}</p>

  const name = (uid: string | null) => (uid ? (names.get(uid) ?? uid) : '—')

  return (
    <section className="detail">
      <button className="link" onClick={onBack}>
        ← All requests
      </button>

      <h2>
        {r.values.description || <em>Untitled</em>}{' '}
        <span className={`badge ${r.status.toLowerCase()}`}>{r.status}</span>
      </h2>

      <dl>
        <dt>Reference</dt>
        <dd>{r.id}</dd>
        <dt>Requested by</dt>
        <dd>{name(r.requesterId)}</dd>
        <dt>Type</dt>
        <dd>{r.values.expenseType}</dd>
        <dt>Amount</dt>
        <dd>{money(r.values.amountCents)}</dd>
        {r.values.billable && (
          <>
            <dt>Client</dt>
            <dd>{r.values.client}</dd>
          </>
        )}
        {r.values.additionalJustification && (
          <>
            <dt>Justification</dt>
            <dd>{r.values.additionalJustification}</dd>
          </>
        )}
        {r.values.otherReason && (
          <>
            <dt>Reason</dt>
            <dd>{r.values.otherReason}</dd>
          </>
        )}
      </dl>

      {r.steps?.length > 0 && (
        <div className="steps">
          {r.steps.map((s, i) => (
            <span key={i} className={`step ${s.status.toLowerCase()}`}>
              {s.name}: {name(s.approverId)}
            </span>
          ))}
        </div>
      )}

      {error && <p className="error">{error}</p>}

      <div className="actions">
        {canEdit(r, me.id) && (
          <>
            <button onClick={() => onEdit(r.id)}>Edit</button>
            <button
              className="primary"
              onClick={() => act(() => submitRequest(me.id, r.id))}
            >
              Submit
            </button>
          </>
        )}

        {canAct(r, me.id) && (
          <>
            <input
              type="text"
              placeholder="Comment (optional)"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
            <button
              className="primary"
              onClick={() => act(() => approveRequest(me.id, r.id, comment))}
            >
              Approve
            </button>
            <button
              className="danger"
              onClick={() => act(() => rejectRequest(me.id, r.id, comment))}
            >
              Reject
            </button>
          </>
        )}
      </div>

      <h3>History</h3>
      <ol className="timeline">
        {(r.timeline ?? []).map((e, i) => (
          <li key={i}>
            <strong>{name(e.actorId)}</strong> {said[e.type]}
            <small> · {when(e.at)}</small>
            {e.comment && <blockquote>{e.comment}</blockquote>}
          </li>
        ))}
      </ol>
    </section>
  )
}
