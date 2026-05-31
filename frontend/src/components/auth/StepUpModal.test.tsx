/**
 * Tests for StepUpModal — the TOTP step-up challenge gate.
 *
 * Key assertions:
 *   - Modal does not render when store.open === false
 *   - Modal renders when store.open === true
 *   - Confirm button is disabled until a 6-digit code is entered
 *   - Valid code submission calls submitStepUpCode and then store.resolve()
 *   - Invalid code (400 / invalid_code) shows an error, does NOT call resolve()
 *   - Cancel button calls store.reject()
 *   - X (aria-label Cancel) button calls store.reject()
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, beforeEach, afterEach } from 'vitest'

import { StepUpModal } from './StepUpModal'
import { useStepUpStore } from '../../store/stepup'
import * as apiModule from '../../lib/api'

// ── Helpers ────────────────────────────────────────────────────────────────────

/** Open the modal by calling the real store.prompt() so state is coherent. */
async function openModal() {
  // Attach a catch so that cancellation (reject) doesn't produce unhandled rejections.
  useStepUpStore.getState().prompt().catch(() => undefined)
  // Let the synchronous Zustand update settle
  await Promise.resolve()
}

function renderModal() {
  return render(<StepUpModal />)
}

// ── Setup ─────────────────────────────────────────────────────────────────────

beforeEach(() => {
  // Fully reset the store to its initial closed state
  useStepUpStore.setState({
    open: false,
    _pendingPromise: null,
    _resolve: null,
    _reject: null,
  } as Parameters<typeof useStepUpStore.setState>[0])
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('StepUpModal — visibility', () => {
  it('should not render when store.open is false', () => {
    renderModal()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('should render the dialog when store.open is true', async () => {
    await openModal()
    renderModal()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('should display the "Confirm identity" heading', async () => {
    await openModal()
    renderModal()
    expect(screen.getByText(/confirm identity/i)).toBeInTheDocument()
  })
})

describe('StepUpModal — Confirm button gate', () => {
  it('should have Confirm button disabled when input is empty', async () => {
    await openModal()
    renderModal()
    expect(screen.getByRole('button', { name: /confirm/i })).toBeDisabled()
  })

  it('should keep Confirm button disabled when fewer than 6 digits entered', async () => {
    await openModal()
    renderModal()
    await userEvent.type(screen.getByPlaceholderText('000000'), '12345')
    expect(screen.getByRole('button', { name: /confirm/i })).toBeDisabled()
  })

  it('should enable Confirm button only when exactly 6 digits are entered', async () => {
    await openModal()
    renderModal()
    await userEvent.type(screen.getByPlaceholderText('000000'), '123456')
    expect(screen.getByRole('button', { name: /confirm/i })).not.toBeDisabled()
  })
})

describe('StepUpModal — successful submission', () => {
  it('should call submitStepUpCode with the 6-digit code and close the modal', async () => {
    vi.spyOn(apiModule, 'submitStepUpCode').mockResolvedValue(undefined)

    await openModal()
    renderModal()

    await userEvent.type(screen.getByPlaceholderText('000000'), '654321')
    await userEvent.click(screen.getByRole('button', { name: /confirm/i }))

    await waitFor(() => {
      expect(apiModule.submitStepUpCode).toHaveBeenCalledWith('654321')
    })
    // After successful submission, the store should be closed (resolve was called)
    await waitFor(() => {
      expect(useStepUpStore.getState().open).toBe(false)
    })
  })
})

describe('StepUpModal — error handling', () => {
  it('should show "Invalid code" error message when backend returns invalid_code', async () => {
    vi.spyOn(apiModule, 'submitStepUpCode').mockRejectedValue(
      new apiModule.ApiError(400, { error: 'invalid_code' }),
    )

    await openModal()
    renderModal()

    await userEvent.type(screen.getByPlaceholderText('000000'), '000000')
    await userEvent.click(screen.getByRole('button', { name: /confirm/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid code/i)).toBeInTheDocument()
    })
    // The dialog must still be open (store is still in open state after error)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('should show "2FA is not enabled" error for 2fa_not_enabled response', async () => {
    vi.spyOn(apiModule, 'submitStepUpCode').mockRejectedValue(
      new apiModule.ApiError(400, { error: '2fa_not_enabled' }),
    )

    await openModal()
    renderModal()

    await userEvent.type(screen.getByPlaceholderText('000000'), '111111')
    await userEvent.click(screen.getByRole('button', { name: /confirm/i }))

    await waitFor(() => {
      expect(screen.getByText(/2FA is not enabled/i)).toBeInTheDocument()
    })
  })

  it('should show generic error for unexpected API error', async () => {
    vi.spyOn(apiModule, 'submitStepUpCode').mockRejectedValue(
      new apiModule.ApiError(400, { error: 'some_unknown_error' }),
    )

    await openModal()
    renderModal()

    await userEvent.type(screen.getByPlaceholderText('000000'), '999999')
    await userEvent.click(screen.getByRole('button', { name: /confirm/i }))

    await waitFor(() => {
      expect(screen.getByText(/verification failed/i)).toBeInTheDocument()
    })
  })
})

describe('StepUpModal — cancellation', () => {
  it('should close the modal when the Cancel (text) button is clicked', async () => {
    await openModal()
    renderModal()

    // Confirm the dialog is showing
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    // Click the text "Cancel" button (not the X icon button)
    // getByText is more specific than getByRole here because the X has aria-label="Cancel" too
    await userEvent.click(screen.getByText('Cancel'))

    // Store state should be closed after reject
    expect(useStepUpStore.getState().open).toBe(false)
  })

  it('should close the modal when the X icon button (aria-label Cancel) is clicked', async () => {
    await openModal()
    renderModal()

    // The X button at the top right
    await userEvent.click(screen.getByLabelText('Cancel'))

    expect(useStepUpStore.getState().open).toBe(false)
  })
})
