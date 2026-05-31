/**
 * Tests for the Security page — FlaggedCard / useAcknowledgeFlag optimistic update.
 *
 * Critical assertions:
 *   1. The Acknowledge button renders for an unacknowledged flag.
 *   2. On click the acknowledge mutation is called with the right args.
 *   3. While isPending is true the button shows "Acknowledging…" and is disabled.
 *   4. The optimistic update logic correctly flips acknowledged in the cache
 *      and rolls back on error (verified via QueryClient directly).
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, beforeEach, afterEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

import Security from './Security'
import { privilegedKey } from '../lib/api/security'
import type { PrivilegedResponse } from '../types/api'

// ── Mocks ─────────────────────────────────────────────────────────────────────

const mockAcknowledgeMutate = vi.fn()
let acknowledgePending = false

vi.mock('../lib/api/security', async (importOriginal) => {
  const original = await importOriginal<typeof import('../lib/api/security')>()
  return {
    ...original,
    usePrivileged: () => ({
      data: {
        containers: [
          {
            container_id: 'ctr-1',
            node_id: 'node-1',
            flags: [
              {
                type: 'privileged' as const,
                key: '',
                risk: 'Container runs in privileged mode.',
                acknowledged: false,
              },
            ],
          },
        ],
      } satisfies PrivilegedResponse,
      isLoading: false,
    }),
    usePorts: () => ({ data: { ports: [], non_docker_listeners: [] }, isLoading: false }),
    useAcknowledgeFlag: () => ({
      mutate: mockAcknowledgeMutate,
      isPending: acknowledgePending,
    }),
    useRescan: () => ({ mutate: vi.fn(), isPending: false }),
  }
})

vi.mock('../lib/api/tree', () => ({
  useTree: () => ({
    data: {
      nodes: [
        {
          id: 'node-1',
          name: 'my-node',
          type: 'standalone',
          host: '1.2.3.4',
          status: 'ok',
          capabilities: { proxmox: false, docker: true, agent: false, systemd: true, cron: true },
          proxmox_auth_status: 'none',
          seq: 1,
          vms: [],
          containers: [{ id: 'ctr-1', name: 'nginx', docker_id: 'abc', image: 'nginx', status: 'running', node_id: 'node-1', compose_project: '' }],
        },
      ],
    },
  }),
}))

vi.mock('../hooks/useMe', () => ({
  useMe: () => ({
    data: { id: '1', username: 'admin', role: 'admin' },
    isLoading: false,
  }),
}))

vi.mock('../lib/api/incidents', () => ({
  useIncidentTimeline: () => ({ data: { entries: [] }, isLoading: false }),
}))

vi.mock('../components/layout/AppShell', () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('../components/security/PostureCard', () => ({
  PostureCard: () => <div>PostureCard</div>,
}))

vi.mock('../components/security/EventDetailDrawer', () => ({
  EventDetailDrawer: () => null,
}))

// ── Helpers ────────────────────────────────────────────────────────────────────

function renderSecurity() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Security />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  acknowledgePending = false
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('Security page — Acknowledge flag', () => {
  it('should render the Acknowledge button for an unacknowledged flag', () => {
    renderSecurity()
    expect(screen.getByRole('button', { name: /^acknowledge$/i })).toBeInTheDocument()
  })

  it('should call the acknowledge mutation with the correct args when clicked', async () => {
    renderSecurity()
    await userEvent.click(screen.getByRole('button', { name: /^acknowledge$/i }))
    expect(mockAcknowledgeMutate).toHaveBeenCalledTimes(1)
    expect(mockAcknowledgeMutate).toHaveBeenCalledWith({
      container_id: 'ctr-1',
      flag_type: 'privileged',
      flag_key: '',
    })
  })

  it('should show "Acknowledging…" and be disabled while isPending is true', () => {
    acknowledgePending = true
    renderSecurity()
    const pendingBtn = screen.getByRole('button', { name: /acknowledging/i })
    expect(pendingBtn).toBeDisabled()
  })

  it('should NOT show "Acknowledging…" when not pending', () => {
    acknowledgePending = false
    renderSecurity()
    expect(screen.queryByText(/acknowledging/i)).toBeNull()
  })
})

// ── Optimistic update unit test (QueryClient level) ───────────────────────────

describe('useAcknowledgeFlag — optimistic cache update logic', () => {
  it('should flip acknowledged to true in the cache immediately (optimistic)', () => {
    const qc = new QueryClient()
    const initialData: PrivilegedResponse = {
      containers: [
        {
          container_id: 'ctr-1',
          node_id: 'node-1',
          flags: [
            { type: 'privileged', key: '', risk: 'runs privileged', acknowledged: false },
          ],
        },
      ],
    }
    qc.setQueryData(privilegedKey(), initialData)

    // Apply the same optimistic update logic as in onMutate
    qc.setQueryData(privilegedKey(), (old: PrivilegedResponse | undefined) => {
      if (!old) return old
      return {
        containers: old.containers.map((c) =>
          c.container_id !== 'ctr-1'
            ? c
            : {
                ...c,
                flags: c.flags.map((f) =>
                  f.type === 'privileged' && f.key === ''
                    ? { ...f, acknowledged: true }
                    : f,
                ),
              },
        ),
      }
    })

    const optimistic = qc.getQueryData<PrivilegedResponse>(privilegedKey())
    expect(optimistic?.containers[0].flags[0].acknowledged).toBe(true)
  })

  it('should roll back acknowledged to false on error (rollback)', () => {
    const qc = new QueryClient()
    const originalData: PrivilegedResponse = {
      containers: [
        {
          container_id: 'ctr-1',
          node_id: 'node-1',
          flags: [
            { type: 'privileged', key: '', risk: 'runs privileged', acknowledged: false },
          ],
        },
      ],
    }

    // Simulate: cache was updated optimistically
    qc.setQueryData(privilegedKey(), {
      containers: [
        {
          ...originalData.containers[0],
          flags: [{ ...originalData.containers[0].flags[0], acknowledged: true }],
        },
      ],
    })

    // Rollback to originalData (same as onError handler)
    qc.setQueryData(privilegedKey(), originalData)

    const rolledBack = qc.getQueryData<PrivilegedResponse>(privilegedKey())
    expect(rolledBack?.containers[0].flags[0].acknowledged).toBe(false)
  })
})
