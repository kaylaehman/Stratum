import { useEffect, useState } from 'react'
import { Loader, Download, X } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { readFile, downloadFile } from '../../lib/api/fs'
import { resolveLanguage } from '../../lib/codemirror'
import type { FsEntry } from '../../types/api'

interface FileViewerProps {
  nodeId: string
  dirPath: string
  entry: FsEntry
  onClose: () => void
  onEditRequest?: () => void
  isAdmin: boolean
}

export function FileViewer({
  nodeId,
  dirPath,
  entry,
  onClose,
  onEditRequest,
  isAdmin,
}: FileViewerProps) {
  const filePath =
    dirPath === '/' ? `/${entry.name}` : `${dirPath}/${entry.name}`

  const [content, setContent] = useState<string | null>(null)
  const [tooLarge, setTooLarge] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    setContent(null)
    setTooLarge(false)

    void readFile(nodeId, filePath).then((r) => {
      if (cancelled) return
      if (r.tooLarge) {
        setTooLarge(true)
      } else {
        setContent(r.content)
      }
      setLoading(false)
    }).catch((e: unknown) => {
      if (cancelled) return
      setError(e instanceof Error ? e.message : 'Failed to load file')
      setLoading(false)
    })

    void resolveLanguage(entry.name).then((lang) => {
      if (!cancelled) setLangExt(lang)
    })

    return () => { cancelled = true }
  }, [nodeId, filePath, entry.name])

  return (
    <div
      className="flex flex-col flex-1 min-h-0"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-2 px-3 py-2 shrink-0"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <span
          className="font-mono text-xs flex-1 truncate"
          style={{ color: 'var(--text-primary)' }}
          title={filePath}
        >
          {filePath}
        </span>
        {isAdmin && onEditRequest && !tooLarge && !loading && (
          <button
            type="button"
            onClick={onEditRequest}
            className="text-xs px-2 py-0.5"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Edit
          </button>
        )}
        <button
          type="button"
          onClick={() => downloadFile(nodeId, filePath)}
          className="text-xs px-2 py-0.5 flex items-center gap-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
          title="Download"
        >
          <Download size={11} />
          Download
        </button>
        <button
          type="button"
          onClick={onClose}
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
            padding: '2px',
          }}
          title="Close"
        >
          <X size={14} />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-auto">
        {loading && (
          <div className="flex items-center gap-2 px-4 py-6">
            <Loader
              size={13}
              className="animate-spin"
              style={{ color: 'var(--accent)' }}
            />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading...
            </span>
          </div>
        )}

        {error && (
          <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
            {error}
          </div>
        )}

        {tooLarge && !loading && (
          <div className="flex flex-col items-start gap-3 px-4 py-6">
            <p className="text-xs" style={{ color: 'var(--status-warn)' }}>
              File too large to preview (&gt;5 MB).
            </p>
            <button
              type="button"
              onClick={() => downloadFile(nodeId, filePath)}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Download size={12} />
              Download file
            </button>
          </div>
        )}

        {content !== null && !loading && (
          <CodeMirror
            value={content}
            extensions={langExt ? [langExt] : []}
            editable={false}
            basicSetup={{
              lineNumbers: true,
              foldGutter: false,
              highlightActiveLine: false,
            }}
            theme="dark"
            style={{ fontSize: '12px', fontFamily: "'Space Mono', monospace" }}
          />
        )}
      </div>
    </div>
  )
}
