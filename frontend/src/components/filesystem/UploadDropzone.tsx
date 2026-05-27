import { useCallback, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  Upload,
  X,
  Loader,
  CheckCircle,
  AlertCircle,
  RotateCcw,
} from 'lucide-react'
import { useCan } from '../../lib/roles'
import {
  runChunkedUpload,
  cancelUpload,
  FileExistsError,
  UploadAbortedError,
} from '../../lib/upload'
import { dirKey } from '../../lib/api/fs'

// ---- Types ----

type UploadStatus =
  | 'queued'
  | 'uploading'
  | 'paused'
  | 'done'
  | 'error'
  | 'cancelled'

interface UploadItem {
  id: string
  file: File
  targetPath: string
  status: UploadStatus
  received: number
  error: string | null
  /** Set when finish returned 409 file_exists */
  awaitingOverwriteConfirm: boolean
  abortController: AbortController | null
}

interface UploadDropzoneProps {
  nodeId: string
  destDir: string
  onComplete: () => void
}

// ---- Helper ----

function joinPath(dir: string, name: string): string {
  if (dir === '/') return `/${name}`
  return `${dir}/${name}`
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(1)} MB`
}

// ---- Progress bar ----

function ProgressBar({ pct }: { pct: number }) {
  return (
    <div
      style={{
        height: '2px',
        width: '100%',
        background: 'var(--border-subtle)',
        borderRadius: '1px',
        overflow: 'hidden',
        marginTop: '4px',
      }}
    >
      <div
        style={{
          height: '100%',
          width: `${pct}%`,
          background: 'var(--accent)',
          transition: 'width 0.1s linear',
          borderRadius: '1px',
        }}
      />
    </div>
  )
}

// ---- Main component ----

export function UploadDropzone({ nodeId, destDir, onComplete }: UploadDropzoneProps) {
  const { isAdmin } = useCan()
  const qc = useQueryClient()
  const [items, setItems] = useState<UploadItem[]>([])
  const [dragging, setDragging] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  // Track in-flight items so we can cancel
  const itemsRef = useRef<UploadItem[]>([])

  function patchItem(id: string, patch: Partial<UploadItem>) {
    setItems((prev) => {
      const next = prev.map((it) =>
        it.id === id ? { ...it, ...patch } : it,
      )
      itemsRef.current = next
      return next
    })
  }

  const runUpload = useCallback(
    async (item: UploadItem, overwrite = false) => {
      patchItem(item.id, { status: 'uploading', error: null, awaitingOverwriteConfirm: false })

      const ac = new AbortController()
      patchItem(item.id, { abortController: ac })

      try {
        await runChunkedUpload(
          {
            nodeId,
            file: item.file,
            targetPath: item.targetPath,
            onProgress: (received) => {
              patchItem(item.id, { received })
            },
            signal: ac.signal,
          },
          overwrite,
        )

        patchItem(item.id, {
          status: 'done',
          received: item.file.size,
          abortController: null,
        })
        void qc.invalidateQueries({ queryKey: dirKey(nodeId, destDir) })
        onComplete()
      } catch (err) {
        if (err instanceof UploadAbortedError) {
          patchItem(item.id, { status: 'cancelled', abortController: null })
          void cancelUpload(nodeId, item.targetPath)
        } else if (err instanceof FileExistsError) {
          patchItem(item.id, {
            status: 'paused',
            awaitingOverwriteConfirm: true,
            abortController: null,
          })
        } else {
          patchItem(item.id, {
            status: 'error',
            error: err instanceof Error ? err.message : 'Upload failed',
            abortController: null,
          })
        }
      }
    },
    [nodeId, destDir, qc, onComplete],
  )

  const enqueueFiles = useCallback(
    (files: File[]) => {
      const newItems: UploadItem[] = files.map((f) => ({
        id: `${Date.now()}-${Math.random()}`,
        file: f,
        targetPath: joinPath(destDir, f.name),
        status: 'queued',
        received: 0,
        error: null,
        awaitingOverwriteConfirm: false,
        abortController: null,
      }))

      setItems((prev) => {
        const next = [...prev, ...newItems]
        itemsRef.current = next
        return next
      })

      // Upload sequentially
      void (async () => {
        for (const item of newItems) {
          await runUpload(item)
        }
      })()
    },
    [destDir, runUpload],
  )

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragging(false)
    const files = Array.from(e.dataTransfer.files)
    if (files.length) enqueueFiles(files)
  }

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? [])
    if (files.length) enqueueFiles(files)
    if (inputRef.current) inputRef.current.value = ''
  }

  function handleCancel(item: UploadItem) {
    if (item.abortController) {
      item.abortController.abort()
    } else if (item.status === 'queued' || item.status === 'paused') {
      patchItem(item.id, { status: 'cancelled' })
      void cancelUpload(nodeId, item.targetPath)
    }
  }

  function handleResume(item: UploadItem) {
    void runUpload(item)
  }

  function handleOverwriteConfirm(item: UploadItem) {
    void runUpload(item, true)
  }

  function handleOverwriteDecline(item: UploadItem) {
    patchItem(item.id, { status: 'cancelled', awaitingOverwriteConfirm: false })
    void cancelUpload(nodeId, item.targetPath)
  }

  if (!isAdmin) return null

  const allSettled =
    items.length > 0 &&
    items.every((i) => i.status === 'done' || i.status === 'error' || i.status === 'cancelled')

  return (
    <div className="flex flex-col gap-2">
      {/* Drop zone */}
      <div
        onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
        className="flex flex-col items-center justify-center gap-2 px-4 py-6 cursor-pointer"
        style={{
          border: `2px dashed ${dragging ? 'var(--accent)' : 'var(--border-default)'}`,
          borderRadius: '4px',
          backgroundColor: dragging ? 'var(--accent-glow)' : 'var(--bg-elevated)',
          transition: 'all 0.15s',
        }}
      >
        <Upload
          size={18}
          style={{ color: dragging ? 'var(--accent)' : 'var(--text-muted)' }}
        />
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Drop files here or click to select
        </p>
        <p className="text-xs font-mono" style={{ color: 'var(--text-muted)', opacity: 0.6 }}>
          max 500 MB per file — uploads resume on re-drop
        </p>
        <input
          ref={inputRef}
          type="file"
          multiple
          style={{ display: 'none' }}
          onChange={handleInputChange}
        />
      </div>

      {/* Queue */}
      {items.length > 0 && (
        <div className="flex flex-col gap-1">
          {items.map((item) => {
            const pct =
              item.file.size > 0
                ? Math.round((item.received / item.file.size) * 100)
                : 0

            return (
              <div
                key={item.id}
                className="flex flex-col gap-1 px-2 py-2"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                {/* Row 1: icon + name + status + actions */}
                <div className="flex items-center gap-2">
                  {/* Status icon */}
                  {item.status === 'done' && (
                    <CheckCircle
                      size={12}
                      style={{ color: 'var(--status-ok)', flexShrink: 0 }}
                    />
                  )}
                  {(item.status === 'error') && (
                    <AlertCircle
                      size={12}
                      style={{ color: 'var(--status-error)', flexShrink: 0 }}
                    />
                  )}
                  {item.status === 'uploading' && (
                    <Loader
                      size={12}
                      className="animate-spin"
                      style={{ color: 'var(--accent)', flexShrink: 0 }}
                    />
                  )}
                  {(item.status === 'queued' || item.status === 'paused' || item.status === 'cancelled') && (
                    <span
                      style={{
                        width: 12,
                        height: 12,
                        borderRadius: '50%',
                        border: '2px solid var(--border-default)',
                        display: 'inline-block',
                        flexShrink: 0,
                      }}
                    />
                  )}

                  {/* Filename */}
                  <span
                    className="font-mono text-xs flex-1 truncate"
                    style={{ color: 'var(--text-primary)' }}
                    title={item.file.name}
                  >
                    {item.file.name}
                  </span>

                  {/* Size + percent */}
                  <span
                    className="font-mono text-xs"
                    style={{ color: 'var(--text-muted)', flexShrink: 0 }}
                  >
                    {item.status === 'uploading'
                      ? `${pct}% / ${formatSize(item.file.size)}`
                      : formatSize(item.file.size)}
                  </span>

                  {/* Cancel button (active uploads + queued) */}
                  {(item.status === 'uploading' || item.status === 'queued') && (
                    <button
                      type="button"
                      onClick={() => handleCancel(item)}
                      title="Cancel"
                      style={{
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        padding: '0 2px',
                        display: 'flex',
                        alignItems: 'center',
                        color: 'var(--text-muted)',
                        flexShrink: 0,
                      }}
                    >
                      <X size={11} />
                    </button>
                  )}

                  {/* Resume button */}
                  {item.status === 'error' && (
                    <button
                      type="button"
                      onClick={() => handleResume(item)}
                      title="Resume"
                      style={{
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        padding: '0 2px',
                        display: 'flex',
                        alignItems: 'center',
                        color: 'var(--accent)',
                        flexShrink: 0,
                      }}
                    >
                      <RotateCcw size={11} />
                    </button>
                  )}
                </div>

                {/* Progress bar */}
                {item.status === 'uploading' && (
                  <ProgressBar pct={pct} />
                )}

                {/* Error message */}
                {item.status === 'error' && item.error && (
                  <p
                    className="font-mono text-xs"
                    style={{ color: 'var(--status-error)', paddingLeft: '16px' }}
                  >
                    {item.error}
                  </p>
                )}

                {/* Overwrite confirm */}
                {item.awaitingOverwriteConfirm && (
                  <div
                    className="flex items-center gap-2"
                    style={{ paddingLeft: '16px' }}
                  >
                    <span
                      className="font-mono text-xs"
                      style={{ color: 'var(--status-warn)', flex: 1 }}
                    >
                      {item.file.name} already exists — overwrite?
                    </span>
                    <button
                      type="button"
                      onClick={() => handleOverwriteConfirm(item)}
                      className="font-mono text-xs px-2 py-0.5"
                      style={{
                        background: 'rgba(232,64,64,0.12)',
                        border: '1px solid var(--status-error)',
                        color: 'var(--status-error)',
                        borderRadius: '3px',
                        cursor: 'pointer',
                      }}
                    >
                      Overwrite
                    </button>
                    <button
                      type="button"
                      onClick={() => handleOverwriteDecline(item)}
                      className="font-mono text-xs px-2 py-0.5"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--border-default)',
                        color: 'var(--text-secondary)',
                        borderRadius: '3px',
                        cursor: 'pointer',
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                )}
              </div>
            )
          })}

          {/* Clear completed */}
          {allSettled && (
            <button
              type="button"
              onClick={() => {
                setItems([])
                itemsRef.current = []
              }}
              className="flex items-center gap-1 text-xs self-end"
              style={{
                color: 'var(--text-muted)',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              <X size={11} />
              Clear list
            </button>
          )}
        </div>
      )}
    </div>
  )
}
