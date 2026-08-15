import { useEffect, useState } from 'react'

import { listRequests } from './api'
import { money, when } from './rules'
import type { RequestSummary, User } from './types'

interface Props {
  me: User
  names: Map<string, string>
  onOpen: (id: string) => void
}

export function RequestList({ me, names, onOpen }: Props) {
  const [rows, setRows] = useState<RequestSummary[]>([])
  const [status, setStatus] = useState('')
  const [scope, setScope] = useState('all')
  const [q, setQ] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    listRequests(me.id, { status, scope, q })
      .then((r) => {
        setRows(r)
        setError('')
      })
      .catch((e: Error) => setError(e.message))
  }, [me.id, status, scope, q])

  const name = (id: string | null) => (id ? (names.get(id) ?? id) : '—')

  return (
    <section>
      <div className="filters">
        <select value={scope} onChange={(e) => setScope(e.target.value)}>
          <option value="all">All requests</option>
          <option value="mine">Mine</option>
          <option value="assigned">Waiting on me</option>
        </select>

        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">Any status</option>
          <option value="Draft">Draft</option>
          <option value="Submitted">Submitted</option>
          <option value="Approved">Approved</option>
          <option value="Rejected">Rejected</option>
        </select>

        <input
          type="search"
          placeholder="Search descriptions"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      {error && <p className="error">{error}</p>}

      <table>
        <thead>
          <tr>
            <th>Description</th>
            <th>Type</th>
            <th className="right">Amount</th>
            <th>Requester</th>
            <th>Waiting on</th>
            <th>Status</th>
            <th>Updated</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id} onClick={() => onOpen(r.id)}>
              <td>{r.description || <em>Untitled</em>}</td>
              <td>{r.expenseType}</td>
              <td className="right">{money(r.amountCents)}</td>
              <td>{name(r.requesterId)}</td>
              <td>{name(r.approverId)}</td>
              <td>
                <span className={`badge ${r.status.toLowerCase()}`}>{r.status}</span>
              </td>
              <td>{when(r.updatedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {rows.length === 0 && !error && <p className="empty">Nothing matches those filters.</p>}
    </section>
  )
}
