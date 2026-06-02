import { create } from 'zustand'

type Resolve = () => void
type Reject = (reason?: unknown) => void

/** challenge = user has TOTP, enter a code; enroll = user must set up TOTP
 * before this destructive action is allowed (fail-closed gate). */
export type StepUpMode = 'challenge' | 'enroll'

interface StepUpState {
  open: boolean
  mode: StepUpMode
  // Shared promise for concurrent 428s — all callers await the same challenge
  _pendingPromise: Promise<void> | null
  _resolve: Resolve | null
  _reject: Reject | null
  /** Called by the interceptor on "2fa_required": returns a promise that
   * resolves when the user completes the TOTP challenge, or rejects on cancel. */
  prompt: () => Promise<void>
  /** Called by the interceptor on "totp_enrollment_required": shows an
   * enrollment prompt. The returned promise always rejects (the action cannot
   * proceed until the user enrolls), so the caller surfaces the original error. */
  promptEnroll: () => Promise<void>
  /** Called by StepUpModal on success. */
  resolve: () => void
  /** Called by StepUpModal on cancel. */
  reject: () => void
}

export const useStepUpStore = create<StepUpState>()((set, get) => ({
  open: false,
  mode: 'challenge',
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
    set({ open: true, mode: 'challenge', _pendingPromise: p, _resolve: res!, _reject: rej! })
    return p
  },

  promptEnroll(): Promise<void> {
    const { _pendingPromise } = get()
    if (_pendingPromise !== null) {
      return _pendingPromise
    }
    let res: Resolve
    let rej: Reject
    const p = new Promise<void>((resolve, reject) => {
      res = resolve
      rej = reject
    })
    set({ open: true, mode: 'enroll', _pendingPromise: p, _resolve: res!, _reject: rej! })
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
