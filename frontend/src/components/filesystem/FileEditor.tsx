import { useEffect, useState, useCallback } from 'react'
import { Loader, X, Save, AlertTriangle, Clock, Camera, RotateCcw, GitCompare } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { readFile, writeFile, StaleWriteError } from '../../lib/api/fs'
import { resolveLanguage } from '../../lib/codemirror'
import { DiffView } from './DiffView'
import type { FsEntry } from '../../types/api'
import { useAuthStore } from '../../store/auth'
import {
  useConfigVersions,
  useConfigDrift,
  useSnapshotConfig,
  useRevertConfig,
} from '../../lib/api/configversion'
import type { ConfigVersion } from '../../lib/api/configversion'

interface FileEditorProps {
  nodeId: string
  dirPath: string
  entry: FsEntry
  onClose: () => void
}

type ActiveTab = 'editor' | 'history'

export function FileEditor({ nodeId, dirPath, entry, onClose }: FileEditorProps) {
  const filePath =
    dirPath === '/' ? `/${entry.name}` : `${dirPath}/${entry.name}`

  const [content, setContent] = useState<string>('')
  const [baseline, setBaseline] = useState<string>('') // on-disk content, for the save diff
  const [reviewing, setReviewing] = useState(false)
  const [lastModified, setLastModified] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [staleConflict, setStaleConflict] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)

  // Config-versioning state
  const [activeTab, setActiveTab] = useState<ActiveTab>('editor')
  const [selectedVersion, setSelectedVersion] = useState<ConfigVersion | null>(null)
  const [revertCandidate, setRevertCandidate] = useState<ConfigVersion | null>(null)

  const user = useAuthStore((s) => s.user)
  const isAdmin = user?.role === 'admin'

  const versionsQuery = useConfigVersions(nodeId, filePath, activeTab === 'history')
  const driftQuery = useConfigDrift(nodeId, filePath)
  const snapshotMut = useSnapshotConfig(nodeId, filePath)
  const revertMut = useRevertConfig(nodeId, filePath)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(null)

    void readFile(nodeId, filePath).then((r) => {
      if (cancelled) return
      setContent(r.content)
      setBaseline(r.content)
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
      setBaseline(content)
      setDirty(false)
      setReviewing(false)
    } catch (e) {
      if (e instanceof StaleWriteError) {
        setStaleConflict(true)
        setReviewing(false)
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
      setBaseline(r.content)
      setLastModified(r.lastModified)
      setDirty(false)
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : 'Failed to reload')
    } finally {
      setLoading(false)
    }
  }

  const isDrifted = driftQuery.data?.is_drifted === true && driftQuery.data?.has_snapshot === true

  function handleTabSwitch(tab: ActiveTab) {
    setActiveTab(tab)
    setSelectedVersion(null)
    setRevertCandidate(null)
    if (reviewing) setReviewing(false)
  }

  function handleSelectVersion(v: ConfigVersion) {
    setSelectedVersion(v === selectedVersion ? null : v)
    setRevertCandidate(null)
  }

  async function handleRevert() {
    if (!revertCandidate) return
    try {
      await revertMut.mutateAsync(revertCandidate.id)
      // Reload current content to reflect the revert
      const r = await readFile(nodeId, filePath)
      setContent(r.content)
      setBaseline(r.content)
      setLastModified(r.lastModified)
      setDirty(false)
      setRevertCandidate(null)
      setSelectedVersion(null)
    } catch {
      // error surfaced via revertMut.error
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

        {/* Drift badge */}
        {isDrifted && (
          <span
            className="flex items-center gap-1 text-xs px-1.5 py-0.5"
            style={{
              background: 'rgba(240,160,32,0.12)',
              border: '1px solid rgba(240,160,32,0.35)',
              color: 'var(--status-warn)',
              borderRadius: '3px',
            }}
            title="On-disk content differs from the last snapshot"
          >
            <GitCompare size={10} />
            Drifted
          </span>
        )}

        {/* Snapshot button */}
        <button
          type="button"
          onClick={() => void snapshotMut.mutateAsync()}
          disabled={snapshotMut.isPending}
          className="flex items-center gap-1 text-xs px-2 py-0.5"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: snapshotMut.isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: snapshotMut.isPending ? 'default' : 'pointer',
          }}
          title="Save a manual snapshot of the current on-disk content"
        >
          {snapshotMut.isPending
            ? <Loader size={11} className="animate-spin" />
            : <Camera size={11} />
          }
          Snapshot
        </button>

        {/* Tab buttons */}
        <button
          type="button"
          onClick={() => handleTabSwitch('editor')}
          className="flex items-center gap-1 text-xs px-2 py-0.5"
          style={{
            background: activeTab === 'editor' ? 'var(--accent-glow)' : 'var(--bg-elevated)',
            border: `1px solid ${activeTab === 'editor' ? 'var(--accent-dim)' : 'var(--border-default)'}`,
            color: activeTab === 'editor' ? 'var(--accent)' : 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Save size={11} />
          Editor
        </button>
        <button
          type="button"
          onClick={() => handleTabSwitch('history')}
          className="flex items-center gap-1 text-xs px-2 py-0.5"
          style={{
            background: activeTab === 'history' ? 'var(--accent-glow)' : 'var(--bg-elevated)',
            border: `1px solid ${activeTab === 'history' ? 'var(--accent-dim)' : 'var(--border-default)'}`,
            color: activeTab === 'history' ? 'var(--accent)' : 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Clock size={11} />
          History
        </button>

        {/* Review & save (editor tab only) */}
        {activeTab === 'editor' && (
          <button
            type="button"
            onClick={() => setReviewing(true)}
            disabled={saving || loading || !dirty || reviewing}
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
            Review &amp; save
          </button>
        )}

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

      {/* ── EDITOR TAB ─────────────────────────────── */}
      {activeTab === 'editor' && (
        <>
          {/* Review-before-save: diff of on-disk baseline vs edits */}
          {reviewing && (
            <div className="flex flex-col flex-1 min-h-0">
              <div
                className="flex items-center gap-2 px-3 py-2 shrink-0"
                style={{ borderBottom: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
              >
                <span className="text-xs uppercase tracking-wider flex-1" style={{ color: 'var(--text-muted)' }}>
                  Review changes
                </span>
                <button
                  type="button"
                  onClick={() => void handleSave()}
                  disabled={saving}
                  className="flex items-center gap-1 text-xs px-2 py-0.5"
                  style={{ background: 'var(--accent-glow)', border: '1px solid var(--accent-dim)', color: 'var(--accent)', borderRadius: '3px', cursor: saving ? 'default' : 'pointer' }}
                >
                  {saving ? <Loader size={11} className="animate-spin" /> : <Save size={11} />}
                  Confirm save
                </button>
                <button
                  type="button"
                  onClick={() => setReviewing(false)}
                  disabled={saving}
                  className="text-xs px-2 py-0.5"
                  style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)', borderRadius: '3px', cursor: 'pointer' }}
                >
                  Cancel
                </button>
              </div>
              <DiffView oldText={baseline} newText={content} />
            </div>
          )}

          {/* Body */}
          <div className="flex-1 overflow-auto" style={{ display: reviewing ? 'none' : undefined }}>
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
                style={{ fontSize: '12px', fontFamily: "'IBM Plex Mono', monospace" }}
              />
            )}
          </div>
        </>
      )}

      {/* ── HISTORY TAB ────────────────────────────── */}
      {activeTab === 'history' && (
        <div className="flex flex-1 min-h-0">
          {/* Snapshot list (left pane) */}
          <div
            className="flex flex-col shrink-0 overflow-y-auto"
            style={{
              width: '220px',
              borderRight: '1px solid var(--border-subtle)',
            }}
          >
            {versionsQuery.isLoading && (
              <div className="flex items-center gap-2 px-3 py-4">
                <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading...</span>
              </div>
            )}
            {versionsQuery.isError && (
              <div className="px-3 py-3 text-xs" style={{ color: 'var(--status-error)' }}>
                Failed to load history.
              </div>
            )}
            {versionsQuery.data && versionsQuery.data.length === 0 && (
              <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                No snapshots yet. Use the Snapshot button to save one.
              </div>
            )}
            {versionsQuery.data?.map((v) => {
              const isSelected = selectedVersion?.id === v.id
              const ts = new Date(v.created_at)
              return (
                <button
                  key={v.id}
                  type="button"
                  onClick={() => handleSelectVersion(v)}
                  className="flex flex-col items-start px-3 py-2 text-left w-full"
                  style={{
                    background: isSelected ? 'var(--accent-glow)' : 'transparent',
                    borderBottom: '1px solid var(--border-subtle)',
                    borderLeft: isSelected ? '2px solid var(--accent)' : '2px solid transparent',
                    cursor: 'pointer',
                  }}
                >
                  <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                    {v.hash.slice(0, 8)}
                  </span>
                  <span className="text-xs" style={{ color: 'var(--text-primary)' }}>
                    {ts.toLocaleDateString()} {ts.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                  <span className="text-xs truncate w-full" style={{ color: 'var(--text-muted)' }}>
                    {v.author}
                  </span>
                </button>
              )
            })}
          </div>

          {/* Right pane: diff or placeholder */}
          <div className="flex flex-col flex-1 min-h-0">
            {!selectedVersion && (
              <div className="px-4 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>
                Select a snapshot to view the diff vs current on-disk content.
              </div>
            )}

            {selectedVersion && (
              <>
                {/* Diff toolbar */}
                <div
                  className="flex items-center gap-2 px-3 py-2 shrink-0"
                  style={{ borderBottom: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
                >
                  <span className="text-xs flex-1" style={{ color: 'var(--text-muted)' }}>
                    Snapshot <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{selectedVersion.hash.slice(0, 8)}</span>
                    {' '}vs current
                  </span>

                  {/* Revert action — admin only */}
                  {isAdmin && (
                    revertCandidate?.id === selectedVersion.id
                      ? (
                        <div className="flex items-center gap-1">
                          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>Confirm revert?</span>
                          <button
                            type="button"
                            onClick={() => void handleRevert()}
                            disabled={revertMut.isPending}
                            className="flex items-center gap-1 text-xs px-2 py-0.5"
                            style={{
                              background: 'rgba(232,64,64,0.12)',
                              border: '1px solid rgba(232,64,64,0.35)',
                              color: 'var(--status-error)',
                              borderRadius: '3px',
                              cursor: revertMut.isPending ? 'default' : 'pointer',
                            }}
                          >
                            {revertMut.isPending
                              ? <Loader size={11} className="animate-spin" />
                              : <RotateCcw size={11} />
                            }
                            Yes, revert
                          </button>
                          <button
                            type="button"
                            onClick={() => setRevertCandidate(null)}
                            className="text-xs px-2 py-0.5"
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
                      )
                      : (
                        <button
                          type="button"
                          onClick={() => setRevertCandidate(selectedVersion)}
                          className="flex items-center gap-1 text-xs px-2 py-0.5"
                          style={{
                            background: 'var(--bg-elevated)',
                            border: '1px solid var(--border-default)',
                            color: 'var(--text-secondary)',
                            borderRadius: '3px',
                            cursor: 'pointer',
                          }}
                          title="Revert the on-disk file to this snapshot (requires admin + step-up)"
                        >
                          <RotateCcw size={11} />
                          Revert to this
                        </button>
                      )
                  )}
                </div>

                {/* Revert error */}
                {revertMut.isError && (
                  <div className="px-3 py-1.5 text-xs shrink-0" style={{ color: 'var(--status-error)' }}>
                    {revertMut.error instanceof Error ? revertMut.error.message : 'Revert failed'}
                  </div>
                )}

                {/* Diff: snapshot content vs current on-disk.
                    The drift endpoint provides content only for the latest snapshot.
                    For older snapshots we show a metadata-only summary. */}
                {driftQuery.data?.snapshot_hash === selectedVersion.hash
                  ? (
                    <DiffView
                      oldText={driftQuery.data.snapshot_content}
                      newText={baseline}
                    />
                  )
                  : (
                    <div className="px-4 py-4 text-xs" style={{ color: 'var(--text-muted)' }}>
                      Full diff is available only for the latest snapshot. Select the topmost entry to compare.
                    </div>
                  )
                }
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
