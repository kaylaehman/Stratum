import { create } from 'zustand'

type Resolve = () => void
type Reject = (reason?: unknown) => void

interface StepUpState {
  open: boolean
  // Shared promise for concurrent 428s — all callers await the same challenge
  _pendingPromise: Promise<void> | null
  _resolve: Resolve | null
  _reject: Reject | null
  /** Called by the interceptor: returns a promise that resolves when the user
   * completes the TOTP challenge, or rejects when they cancel. */
  prompt: () => Promise<void>
  /** Called by StepUpModal on success. */
  resolve: () => void
  /** Called by StepUpModal on cancel. */
  reject: () => void
}

export const useStepUpStore = create<StepUpState>()((set, get) => ({
  open: false,
  _pendingPromise: null,
  _resolve: null,
  _reject: null,

  prompt(): Promise<void> {
    const { _pendingPromise } = get()
    // Coalesce concurrent 428s into one modal
    if (_pendingPromise !== null) {
      return _pendingPromise
    }
    let res: Resolve
    let rej: Reject
    const p = new Promise<void>((resolve, reject) => {
      res = resolve
      rej = reject
    })
    set({ open: true, _pendingPromise: p, _resolve: res!, _reject: rej! })
    return p
  },

  resolve() {
    const { _resolve } = get()
    set({ open: false, _pendingPromise: null, _resolve: null, _reject: null })
    _resolve?.()
  },

  reject() {
    const { _reject } = get()
    set({ open: false, _pendingPromise: null, _resolve: null, _reject: null })
    _reject?.(new Error('step_up_cancelled'))
  },
}))
