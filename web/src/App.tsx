import { useEffect, useMemo, useState } from 'react'

import { getRequest, listClients, listUsers } from './api'
import { RequestDetail } from './RequestDetail'
import { RequestForm } from './RequestForm'
import { RequestList } from './RequestList'
import type { Client, RequestView, User } from './types'

type View =
  | { name: 'list' }
  | { name: 'detail'; id: string }
  | { name: 'form'; existing?: RequestView }

const STORED_USER = 'expense-requests.user'

export function App() {
  const [users, setUsers] = useState<User[]>([])
  const [clients, setClients] = useState<Client[]>([])
  const [meId, setMeId] = useState(() => localStorage.getItem(STORED_USER) ?? '')
  const [view, setView] = useState<View>({ name: 'list' })
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([listUsers(), listClients()])
      .then(([u, c]) => {
        setUsers(u)
        setClients(c)
        setMeId((current) => current || (u[0]?.id ?? ''))
      })
      .catch((e: Error) => setError(e.message))
  }, [])

  useEffect(() => {
    if (meId) localStorage.setItem(STORED_USER, meId)
  }, [meId])

  // The API sends identifiers; names are resolved here, from the list the
  // picker needs anyway.
  const names = useMemo(
    () => new Map(users.map((u) => [u.id, u.name])),
    [users],
  )

  const me = users.find((u) => u.id === meId)

  if (error) return <p className="error page">{error}</p>
  if (!me) return <p className="page">Loading…</p>

  return (
    <div className="page">
      <header>
        <h1>Expense Requests</h1>

        <div className="who">
          <label>
            Acting as{' '}
            <select value={meId} onChange={(e) => setMeId(e.target.value)}>
              {users.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name} ({u.role})
                </option>
              ))}
            </select>
          </label>

          <button className="primary" onClick={() => setView({ name: 'form' })}>
            New request
          </button>
        </div>
      </header>

      {view.name === 'list' && (
        <RequestList
          me={me}
          names={names}
          onOpen={(id) => setView({ name: 'detail', id })}
        />
      )}

      {view.name === 'detail' && (
        <RequestDetail
          me={me}
          names={names}
          id={view.id}
          onBack={() => setView({ name: 'list' })}
          onEdit={async (id) =>
            setView({ name: 'form', existing: await getRequest(me.id, id) })
          }
        />
      )}

      {view.name === 'form' && (
        <RequestForm
          me={me}
          clients={clients}
          existing={view.existing}
          onDone={(id) => setView({ name: 'detail', id })}
          onCancel={() => setView({ name: 'list' })}
        />
      )}
    </div>
  )
}
