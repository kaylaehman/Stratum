import { useCallback, useRef, useState } from 'react'
import { Upload, X, CheckCircle, AlertCircle } from 'lucide-react'
import { uploadFile } from '../../lib/api/fs'

interface UploadItem {
  file: File
  progress: number
  done: boolean
  error: string | null
}

interface UploadDropzoneProps {
  nodeId: string
  destDir: string
  onComplete: () => void
}

export function UploadDropzone({ nodeId, destDir, onComplete }: UploadDropzoneProps) {
  const [items, setItems] = useState<UploadItem[]>([])
  const [dragging, setDragging] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  function updateItem(index: number, patch: Partial<UploadItem>) {
    setItems((prev) => {
      const next = [...prev]
      next[index] = { ...next[index], ...patch }
      return next
    })
  }

  const processFiles = useCallback(
    async (files: File[]) => {
      const startIdx = items.length
      const newItems: UploadItem[] = files.map((f) => ({
        file: f,
        progress: 0,
        done: false,
        error: null,
      }))
      setItems((prev) => [...prev, ...newItems])

      await Promise.all(
        files.map(async (file, i) => {
          const idx = startIdx + i
          try {
            await uploadFile(nodeId, destDir, file, (p) => {
              updateItem(idx, {
                progress: Math.round((p.loaded / p.total) * 100),
              })
            })
            updateItem(idx, { done: true, progress: 100 })
          } catch (e) {
            updateItem(idx, {
              error: e instanceof Error ? e.message : 'Upload failed',
            })
          }
        }),
      )
      onComplete()
    },
    [nodeId, destDir, items.length, onComplete],
  )

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragging(false)
    const files = Array.from(e.dataTransfer.files)
    if (files.length) void processFiles(files)
  }

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? [])
    if (files.length) void processFiles(files)
    if (inputRef.current) inputRef.current.value = ''
  }

  const allDone = items.length > 0 && items.every((i) => i.done || i.error)

  return (
    <div className="flex flex-col gap-2">
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
        <Upload size={18} style={{ color: dragging ? 'var(--accent)' : 'var(--text-muted)' }} />
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Drop files here or click to select
        </p>
        <input
          ref={inputRef}
          type="file"
          multiple
          style={{ display: 'none' }}
          onChange={handleInputChange}
        />
      </div>

      {items.length > 0 && (
        <div className="flex flex-col gap-1">
          {items.map((item, i) => (
            <div
              key={i}
              className="flex items-center gap-2 px-2 py-1.5"
              style={{
                backgroundColor: 'var(--bg-surface)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
              }}
            >
              {item.done && !item.error && (
                <CheckCircle size={12} style={{ color: 'var(--status-ok)', flexShrink: 0 }} />
              )}
              {item.error && (
                <AlertCircle size={12} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
              )}
              {!item.done && !item.error && (
                <div
                  style={{
                    width: 12,
                    height: 12,
                    borderRadius: '50%',
                    border: '2px solid var(--accent)',
                    borderTopColor: 'transparent',
                    animation: 'spin 0.6s linear infinite',
                    flexShrink: 0,
                  }}
                />
              )}
              <span
                className="font-mono text-xs flex-1 truncate"
                style={{ color: 'var(--text-primary)' }}
              >
                {item.file.name}
              </span>
              {!item.done && !item.error && (
                <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                  {item.progress}%
                </span>
              )}
              {item.error && (
                <span className="text-xs" style={{ color: 'var(--status-error)' }}>
                  {item.error}
                </span>
              )}
            </div>
          ))}
          {allDone && (
            <button
              type="button"
              onClick={() => setItems([])}
              className="flex items-center gap-1 text-xs self-end"
              style={{ color: 'var(--text-muted)', background: 'none', border: 'none', cursor: 'pointer' }}
            >
              <X size={11} />
              Clear
            </button>
          )}
        </div>
      )}
    </div>
  )
}
