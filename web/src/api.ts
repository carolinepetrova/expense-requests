import type {
  Client,
  FieldError,
  RequestSummary,
  RequestView,
  User,
  Values,
} from './types'

const BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

/** ApiError carries the field list a 422 comes back with, so the form can mark
 *  every bad field at once rather than showing one message at a time. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fieldErrors: FieldError[]

  constructor(status: number, code: string, message: string, fieldErrors: FieldError[]) {
    super(message)
    this.status = status
    this.code = code
    this.fieldErrors = fieldErrors
  }

  /** byField turns the list into something a form can look up per input. */
  byField(): Record<string, string> {
    return Object.fromEntries(this.fieldErrors.map((f) => [f.field, f.message]))
  }
}

async function call<T>(
  path: string,
  init: { method?: string; body?: unknown; actor?: string } = {},
): Promise<T> {
  const headers: Record<string, string> = {}
  if (init.actor) headers['X-User-Id'] = init.actor
  if (init.body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetch(`${BASE}${path}`, {
    method: init.method ?? 'GET',
    headers,
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  })

  if (!res.ok) {
    const payload = await res.json().catch(() => ({}))
    throw new ApiError(
      res.status,
      payload.error ?? 'error',
      payload.message ?? res.statusText,
      payload.fieldErrors ?? [],
    )
  }

  return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
}

export const listUsers = () => call<User[]>('/api/users')
export const listClients = () => call<Client[]>('/api/clients')

export function listRequests(
  actor: string,
  filter: { status?: string; scope?: string; q?: string },
): Promise<RequestSummary[]> {
  const params = new URLSearchParams()
  if (filter.status) params.set('status', filter.status)
  if (filter.scope && filter.scope !== 'all') params.set('scope', filter.scope)
  if (filter.q) params.set('q', filter.q)

  const query = params.toString()
  return call<RequestSummary[]>(`/api/requests${query ? `?${query}` : ''}`, { actor })
}

export const getRequest = (actor: string, id: string) =>
  call<RequestView>(`/api/requests/${id}`, { actor })

export const createRequest = (actor: string, values: Values) =>
  call<RequestView>('/api/requests', { method: 'POST', actor, body: { values } })

export const updateRequest = (actor: string, id: string, values: Values) =>
  call<RequestView>(`/api/requests/${id}`, { method: 'PATCH', actor, body: { values } })

export const submitRequest = (actor: string, id: string) =>
  call<RequestView>(`/api/requests/${id}/submit`, { method: 'POST', actor })

export const approveRequest = (actor: string, id: string, comment: string) =>
  call<RequestView>(`/api/requests/${id}/approve`, {
    method: 'POST',
    actor,
    body: { comment },
  })

export const rejectRequest = (actor: string, id: string, comment: string) =>
  call<RequestView>(`/api/requests/${id}/reject`, {
    method: 'POST',
    actor,
    body: { comment },
  })
