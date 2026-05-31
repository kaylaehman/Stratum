/**
 * Tests for the Volumes page destructive-confirmation gates.
 *
 * The critical invariant: removeVolume / pruneUnused mutations must NOT be
 * called before the user explicitly confirms in the dialog.
 *
 * Strategy:
 *   - Mock `../lib/api/volumes` so mutation fns are vi.fn()s we can inspect.
 *   - Mock `../lib/api/tree` to return a predictable node list.
 *   - Mock `../hooks/useMe` to return an admin user.
 *   - Render the full Volumes page component (static import after mocks).
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, beforeEach, afterEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

import Volumes from './Volumes'

// ── Mocks ─────────────────────────────────────────────────────────────────────

const mockRemoveVolume = vi.fn()
const mockPruneUnused = vi.fn()

const UNUSED_VOLUME = {
  node_id: 'node-1',
  name: 'orphaned-vol',
  driver: 'local',
  size_bytes: 1024 * 1024 * 200,
  status: 'unused' as const,
  attached_containers: [],
  over_threshold: false,
  samples: [],
}

const ATTACHED_VOLUME = {
  node_id: 'node-1',
  name: 'active-vol',
  driver: 'local',
  size_bytes: 1024 * 1024 * 50,
  status: 'attached' as const,
  attached_containers: ['my-container'],
  over_threshold: false,
  samples: [],
}

vi.mock('../lib/api/volumes', () => ({
  useVolumes: () => ({
    data: { volumes: [UNUSED_VOLUME, ATTACHED_VOLUME] },
    isLoading: false,
  }),
  useRemoveVolume: () => ({
    mutate: mockRemoveVolume,
    isPending: false,
  }),
  usePruneUnusedVolumes: () => ({
    mutate: mockPruneUnused,
    isPending: false,
  }),
}))

vi.mock('../lib/api/tree', () => ({
  useTree: () => ({
    data: { nodes: [{ id: 'node-1', name: 'my-node', containers: [] }] },
  }),
}))

vi.mock('../hooks/useMe', () => ({
  useMe: () => ({
    data: { id: '1', username: 'admin', role: 'admin' },
    isLoading: false,
  }),
}))

// AppShell renders nav + sidebar which depend on many other hooks; stub it.
vi.mock('../components/layout/AppShell', () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

// ── Helpers ────────────────────────────────────────────────────────────────────

function renderVolumes() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Volumes />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ── Single-volume Remove gate ──────────────────────────────────────────────────

describe('Volumes page — single-volume Remove gate', () => {
  it('should not call removeVolume on initial render', () => {
    renderVolumes()
    expect(mockRemoveVolume).not.toHaveBeenCalled()
  })

  it('should show the Remove row-button only for unused volumes (not attached)', () => {
    renderVolumes()
    // orphaned-vol has a Remove button; active-vol does not (it is attached)
    expect(screen.getByTitle('Remove orphaned-vol')).toBeInTheDocument()
    expect(screen.queryByTitle('Remove active-vol')).toBeNull()
  })

  it('should show the confirmation dialog after clicking the per-row Remove button', async () => {
    renderVolumes()
    await userEvent.click(screen.getByTitle('Remove orphaned-vol'))
    // The "this cannot be undone" text is unique to the confirmation dialog
    expect(screen.getByText(/this cannot be undone/i)).toBeInTheDocument()
    // Volume name appears in the dialog (may also appear in the table row — use getAllBy)
    expect(screen.getAllByText('orphaned-vol').length).toBeGreaterThanOrEqual(1)
  })

  it('should NOT call removeVolume when the dialog is opened then Cancelled', async () => {
    renderVolumes()
    await userEvent.click(screen.getByTitle('Remove orphaned-vol'))
    await screen.findByText(/this cannot be undone/i)

    // There is only one Cancel button while the confirm dialog is open
    const cancelBtns = screen.getAllByRole('button', { name: /^cancel$/i })
    await userEvent.click(cancelBtns[0])
    expect(mockRemoveVolume).not.toHaveBeenCalled()
  })

  it('should call removeVolume with the correct args only after dialog confirmation', async () => {
    renderVolumes()
    await userEvent.click(screen.getByTitle('Remove orphaned-vol'))
    await screen.findByText(/this cannot be undone/i)

    // The red "Remove" button inside the dialog (not the row trigger)
    // The row trigger has a title attribute; the dialog button does not.
    const allRemoveBtns = screen.getAllByRole('button', { name: /^remove$/i })
    const dialogConfirm = allRemoveBtns.find((btn) => !btn.hasAttribute('title'))
    if (!dialogConfirm) throw new Error('Dialog Remove button not found')

    await userEvent.click(dialogConfirm)

    expect(mockRemoveVolume).toHaveBeenCalledTimes(1)
    expect(mockRemoveVolume).toHaveBeenCalledWith(
      { nodeId: 'node-1', name: 'orphaned-vol' },
      expect.any(Object),
    )
  })
})

// ── Bulk prune (Remove all unused) gate ───────────────────────────────────────

describe('Volumes page — bulk prune "Remove all unused" gate', () => {
  it('should not call pruneUnused on initial render', () => {
    renderVolumes()
    expect(mockPruneUnused).not.toHaveBeenCalled()
  })

  it('should show the "Remove all unused" toolbar button when admin and unused volumes exist', () => {
    renderVolumes()
    expect(screen.getByRole('button', { name: /remove all unused/i })).toBeInTheDocument()
  })

  it('should show the prune confirmation dialog when "Remove all unused" is clicked', async () => {
    renderVolumes()
    await userEvent.click(screen.getByRole('button', { name: /remove all unused/i }))
    // The dialog heading contains the count
    expect(await screen.findByText(/remove all unused volumes/i)).toBeInTheDocument()
    // The volume should be listed inside the dialog
    expect(screen.getAllByText('orphaned-vol').length).toBeGreaterThanOrEqual(1)
  })

  it('should NOT call pruneUnused when the prune dialog is cancelled', async () => {
    renderVolumes()
    await userEvent.click(screen.getByRole('button', { name: /remove all unused/i }))
    await screen.findByText(/remove all unused volumes/i)

    // There is only one Cancel button while the dialog is open
    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockPruneUnused).not.toHaveBeenCalled()
  })

  it('should call pruneUnused only after confirming in the prune dialog', async () => {
    renderVolumes()
    await userEvent.click(screen.getByRole('button', { name: /remove all unused/i }))
    await screen.findByText(/permanently removed/i)

    // The confirm button shows count e.g. "Remove 1"
    const confirmBtn = screen.getByRole('button', { name: /remove \d+/i })
    await userEvent.click(confirmBtn)

    expect(mockPruneUnused).toHaveBeenCalledTimes(1)
    expect(mockPruneUnused).toHaveBeenCalledWith({}, expect.any(Object))
  })
})
