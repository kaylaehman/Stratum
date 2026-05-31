/**
 * Tests for RemediationPanel + ProposalCard — the approve/execute gate.
 *
 * Critical assertions ("can't fire without its gate"):
 *   1. Viewer sees nothing (non-operator).
 *   2. Operator sees the panel.
 *   3. A proposed proposal shows Approve/Reject — but NOT Execute.
 *   4. An approved proposal shows Execute — but NOT Approve.
 *   5. Approve mutate is called when Approve is clicked.
 *   6. Execute mutate is called when Execute is clicked on an approved proposal.
 *   7. Neither mutation is called before user interaction.
 *   8. Destructive proposals require admin; operator cannot approve them.
 *   9. Terminal proposals show no action buttons.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, beforeEach, afterEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { RemediationPanel } from './RemediationPanel'
import { useAuthStore } from '../../store/auth'
import type { RemediationProposal } from '../../types/api'

// ── Mock the API module ───────────────────────────────────────────────────────
// These are declared at module scope so individual tests can override via
// a closure variable. We keep a mutable ref and replace its value per test.

const mockApproveMutate = vi.fn()
const mockRejectMutate = vi.fn()
const mockExecuteMutate = vi.fn()

// The proposals returned by useProposals are controlled via this ref.
let currentProposals: RemediationProposal[] = []

vi.mock('../../lib/api/remediation', () => ({
  useProposals: () => ({ data: { proposals: currentProposals }, isLoading: false }),
  useApproveProposal: () => ({ mutate: mockApproveMutate, isPending: false, isError: false, error: null }),
  useRejectProposal: () => ({ mutate: mockRejectMutate, isPending: false, isError: false, error: null }),
  useExecuteProposal: () => ({ mutate: mockExecuteMutate, isPending: false, isError: false, error: null }),
}))

// ── Helpers ────────────────────────────────────────────────────────────────────

const BASE_PROPOSAL: RemediationProposal = {
  id: 'prop-1',
  title: 'Fix file permissions',
  rationale: 'Container process cannot read /data',
  commands: ['chmod 644 /data/file.txt'],
  risk_level: 'low',
  status: 'proposed',
  source: 'diagnostic',
  node_id: 'node-1',
  container_id: 'ctr-1',
  created_by: 'admin',
  created_at: new Date().toISOString(),
}

function seedRole(role: 'admin' | 'operator' | 'viewer') {
  useAuthStore.setState({
    accessToken: 'tok',
    user: { id: '1', username: 'user', email: 'u@test.com', role },
  })
}

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <RemediationPanel />
    </QueryClientProvider>,
  )
}

// ── Setup ─────────────────────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  currentProposals = []
  seedRole('admin')
})

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null })
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('RemediationPanel — visibility', () => {
  it('should not render for a viewer (non-operator)', () => {
    seedRole('viewer')
    const { container } = renderPanel()
    expect(container).toBeEmptyDOMElement()
  })

  it('should render the panel heading for an operator', () => {
    seedRole('operator')
    renderPanel()
    expect(screen.getByRole('heading', { name: /remediation proposals/i })).toBeInTheDocument()
  })

  it('should show empty state when no proposals exist', () => {
    renderPanel()
    expect(screen.getByText(/no remediation proposals yet/i)).toBeInTheDocument()
  })
})

describe('RemediationPanel — proposed proposal gate', () => {
  beforeEach(() => {
    currentProposals = [{ ...BASE_PROPOSAL, status: 'proposed' }]
  })

  it('should not have called any mutation before user interaction', () => {
    renderPanel()
    expect(mockApproveMutate).not.toHaveBeenCalled()
    expect(mockRejectMutate).not.toHaveBeenCalled()
    expect(mockExecuteMutate).not.toHaveBeenCalled()
  })

  it('should show Approve button after expanding a proposed proposal (admin)', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    expect(screen.getByRole('button', { name: /approve/i })).toBeInTheDocument()
  })

  it('should show Reject button after expanding a proposed proposal', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    expect(screen.getByRole('button', { name: /reject/i })).toBeInTheDocument()
  })

  it('should NOT show Execute button on a proposed (not yet approved) proposal', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    expect(screen.queryByRole('button', { name: /execute/i })).toBeNull()
  })

  it('should call approveMutate with proposal id when Approve is clicked', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    await userEvent.click(screen.getByRole('button', { name: /approve/i }))
    expect(mockApproveMutate).toHaveBeenCalledWith('prop-1')
    expect(mockExecuteMutate).not.toHaveBeenCalled()
  })

  it('should call rejectMutate with proposal id when Reject is clicked', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    await userEvent.click(screen.getByRole('button', { name: /reject/i }))
    expect(mockRejectMutate).toHaveBeenCalledWith('prop-1')
    expect(mockApproveMutate).not.toHaveBeenCalled()
  })
})

describe('RemediationPanel — approved proposal gate', () => {
  beforeEach(() => {
    currentProposals = [{ ...BASE_PROPOSAL, status: 'approved' }]
  })

  it('should show Execute button for an approved proposal', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    expect(screen.getByRole('button', { name: /execute/i })).toBeInTheDocument()
  })

  it('should NOT show Approve button on an already-approved proposal', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    expect(screen.queryByRole('button', { name: /^approve$/i })).toBeNull()
  })

  it('should call executeMutate when Execute is clicked (low risk)', async () => {
    renderPanel()
    await userEvent.click(screen.getByText('Fix file permissions'))
    await userEvent.click(screen.getByRole('button', { name: /execute/i }))
    expect(mockExecuteMutate).toHaveBeenCalledWith('prop-1')
    expect(mockApproveMutate).not.toHaveBeenCalled()
  })
})

describe('RemediationPanel — destructive proposal gate', () => {
  const destructiveProposed: RemediationProposal = {
    ...BASE_PROPOSAL,
    id: 'prop-destr',
    risk_level: 'destructive',
    title: 'Wipe container filesystem',
    status: 'proposed',
  }

  it('should show the destructive admin+2FA warning label', async () => {
    currentProposals = [destructiveProposed]
    renderPanel()
    await userEvent.click(screen.getByText('Wipe container filesystem'))
    expect(screen.getByText(/destructive.*admin approval.*2fa/i)).toBeInTheDocument()
  })

  it('should show Approve for a destructive proposal when user is admin', async () => {
    seedRole('admin')
    currentProposals = [destructiveProposed]
    renderPanel()
    await userEvent.click(screen.getByText('Wipe container filesystem'))
    expect(screen.getByRole('button', { name: /approve/i })).toBeInTheDocument()
  })

  it('should NOT show Approve for a destructive proposal when user is only operator', async () => {
    seedRole('operator')
    currentProposals = [destructiveProposed]
    renderPanel()
    await userEvent.click(screen.getByText('Wipe container filesystem'))
    expect(screen.queryByRole('button', { name: /^approve$/i })).toBeNull()
  })

  it('should NOT show any action buttons for a rejected (terminal) proposal', async () => {
    currentProposals = [{ ...destructiveProposed, status: 'rejected' }]
    renderPanel()
    await userEvent.click(screen.getByText('Wipe container filesystem'))
    expect(screen.queryByRole('button', { name: /approve/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /reject/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /execute/i })).toBeNull()
  })

  it('should NOT call executeMutate before user clicks Execute on a destructive approved proposal', async () => {
    currentProposals = [{ ...destructiveProposed, status: 'approved' }]
    renderPanel()
    // Just expand — do not click Execute
    await userEvent.click(screen.getByText('Wipe container filesystem'))
    expect(mockExecuteMutate).not.toHaveBeenCalled()
  })
})
