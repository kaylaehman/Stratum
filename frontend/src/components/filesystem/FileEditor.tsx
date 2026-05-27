import { useEffect, useState, useCallback } from 'react'
import { Loader, X, Save, AlertTriangle } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { readFile, writeFile, StaleWriteError } from '../../lib/api/fs'
import { resolveLanguage } from '../../lib/codemirror'
import type { FsEntry } from '../../types/api'

interface FileEditorProps {
  nodeId: string
  dirPath: string
  entry: FsEntry
  onClose: () => void
}

export function FileEditor({ nodeId, dirPath, entry, onClose }: FileEditorProps) {
  const filePath =
    dirPath === '/' ? `/${entry.name}` : `${dirPath}/${entry.name}`

  const [content, setContent] = useState<string>('')
  const [lastModified, setLastModified] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [staleConflict, setStaleConflict] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)

    void readFile(nodeId, filePath).then((r) => {
      if (cancelled) return
      setContent(r.content)
      setLastModified(r.lastModified)
      setLoading(false)
    }).catch((e: unknown) => {
      if (cancelled) return
      setLoadError(e instanceof Error ? e.message : 'Failed to load file')
      setLoading(false)
    })

    void resolveLanguage(entry.name).then((lang) => {
      if (!cancelled) setLangExt(lang)
    })

    return () => { cancelled = true }
  }, [nodeId, filePath, entry.name])

  const handleChange = useCallback((val: string) => {
    setContent(val)
    setDirty(true)
    setStaleConflict(false)
    setSaveError(null)
  }, [])

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    setStaleConflict(false)
    try {
      await writeFile(nodeId, filePath, content, lastModified)
      setDirty(false)
    } catch (e) {
      if (e instanceof StaleWriteError) {
        setStaleConflict(true)
      } else {
        setSaveError(e instanceof Error ? e.message : 'Save failed')
      }
    } finally {
      setSaving(false)
    }
  }

  async function handleReload() {
    setStaleConflict(false)
    setLoading(true)
    try {
      const r = await readFile(nodeId, filePath)
      setContent(r.content)
      setLastModified(r.lastModified)
      setDirty(false)
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : 'Failed to reload')
    } finally {
      setLoading(false)
    }
  }

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
          {dirty && (
            <span style={{ color: 'var(--status-warn)' }}> *</span>
          )}
        </span>
        <button
          type="button"
          onClick={() => void handleSave()}
          disabled={saving || loading || !dirty}
          className="flex items-center gap-1 text-xs px-2 py-0.5"
          style={{
            background: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: saving || !dirty ? 'var(--text-muted)' : 'var(--accent)',
            borderRadius: '3px',
            cursor: saving || !dirty ? 'default' : 'pointer',
          }}
        >
          {saving ? <Loader size={11} className="animate-spin" /> : <Save size={11} />}
          Save
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
          title="Close editor"
        >
          <X size={14} />
        </button>
      </div>

      {/* Stale conflict banner */}
      {staleConflict && (
        <div
          className="flex items-center gap-2 px-3 py-2 text-xs shrink-0"
          style={{
            backgroundColor: 'rgba(240,160,32,0.1)',
            borderBottom: '1px solid var(--border-default)',
            color: 'var(--status-warn)',
          }}
        >
          <AlertTriangle size={12} />
          <span className="flex-1">
            File changed on disk since you opened it.
          </span>
          <button
            type="button"
            onClick={() => void handleReload()}
            className="text-xs px-2 py-0.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Reload from disk
          </button>
        </div>
      )}

      {/* Save error */}
      {saveError && (
        <div
          className="px-3 py-1.5 text-xs shrink-0"
          style={{ color: 'var(--status-error)', borderBottom: '1px solid var(--border-subtle)' }}
        >
          {saveError}
        </div>
      )}

      {/* Body */}
      <div className="flex-1 overflow-auto">
        {loading && (
          <div className="flex items-center gap-2 px-4 py-6">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading...</span>
          </div>
        )}
        {loadError && (
          <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
            {loadError}
          </div>
        )}
        {!loading && !loadError && (
          <CodeMirror
            value={content}
            onChange={handleChange}
            extensions={langExt ? [langExt] : []}
            editable={true}
            basicSetup={{ lineNumbers: true, foldGutter: true }}
            theme="dark"
            style={{ fontSize: '12px', fontFamily: "'Space Mono', monospace" }}
          />
        )}
      </div>
    </div>
  )
}
