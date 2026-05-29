import { useState, useCallback, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Loader,
  FolderPlus,
  Upload as UploadIcon,
  RefreshCw,
} from 'lucide-react'
import { Breadcrumbs } from './Breadcrumbs'
import { FileList } from './FileList'
import { FileViewer } from './FileViewer'
import { FileEditor } from './FileEditor'
import { UploadDropzone } from './UploadDropzone'
import { useDir, useMkdir, useRename, useDeletePath } from '../../lib/api/fs'
import { useAuthStore } from '../../store/auth'
import type { FsEntry } from '../../types/api'

type PanelMode = 'list' | 'view' | 'edit' | 'upload' | 'mkdir'

interface FileBrowserProps {
  nodeId: string
  containerId?: string
}

function joinPath(dir: string, name: string): string {
  if (dir === '/') return `/${name}`
  return `${dir}/${name}`
}

function MkdirPanel({
  onSubmit,
  onCancel,
}: {
  onSubmit: (name: string) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  return (
    <div
      className="flex flex-col gap-3 p-4"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
        New folder
      </p>
      <input
        autoFocus
        type="text"
        placeholder="folder-name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && name.trim()) onSubmit(name.trim())
          if (e.key === 'Escape') onCancel()
        }}
        className="font-mono text-xs px-2 py-1.5"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-primary)',
          borderRadius: '3px',
          outline: 'none',
        }}
      />
      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => { if (name.trim()) onSubmit(name.trim()) }}
          className="text-xs px-3 py-1"
          style={{
            background: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: 'var(--accent)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          Create
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs px-3 py-1"
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
    </div>
  )
}

function DeleteConfirmModal({
  entry,
  onConfirm,
  onCancel,
}: {
  entry: FsEntry
  onConfirm: (recursive: boolean) => void
  onCancel: () => void
}) {
  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50"
      style={{ backgroundColor: 'rgba(0,0,0,0.6)' }}
      onClick={onCancel}
    >
      <div
        className="flex flex-col gap-4 p-5"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '4px',
          minWidth: '320px',
          maxWidth: '460px',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--status-error)' }}>
          Confirm delete
        </p>
        <p className="text-xs" style={{ color: 'var(--text-primary)' }}>
          Delete{' '}
          <span className="font-mono" style={{ color: 'var(--accent)' }}>
            {entry.name}
          </span>
          {entry.is_dir && ' and all its contents'}?
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => onConfirm(entry.is_dir)}
            className="text-xs px-3 py-1.5"
            style={{
              background: 'rgba(232,64,64,0.15)',
              border: '1px solid var(--status-error)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Delete{entry.is_dir ? ' recursively' : ''}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="text-xs px-3 py-1.5"
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
      </div>
    </div>
  )
}

export function FileBrowser({ nodeId, containerId }: FileBrowserProps) {
  const [, setSearchParams] = useSearchParams()
  // Start every filesystem at root. Combined with a per-node `key` on this
  // component (Resources DetailPane), switching to another node's filesystem
  // remounts fresh at "/" rather than carrying over the previous node's path —
  // which usually doesn't exist on the new host and looked like "switching is
  // broken".
  const [path, setPath] = useState('/')
  const [panel, setPanel] = useState<PanelMode>('list')
  const [activeEntry, setActiveEntry] = useState<FsEntry | null>(null)
  const [pendingDelete, setPendingDelete] = useState<FsEntry | null>(null)

  const isAdmin = useAuthStore((s) => s.user?.role === 'admin')

  const { data, isLoading, isError, refetch } = useDir(nodeId, path)
  const mkdir = useMkdir(nodeId, path)
  const rename = useRename(nodeId, path)
  const deletePath = useDeletePath(nodeId, path)

  // Keep URL query param in sync
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('fspath', path)
        return next
      },
      { replace: true },
    )
  }, [path, setSearchParams])

  const navigate = useCallback(
    (newPath: string) => {
      setPath(newPath)
      setPanel('list')
      setActiveEntry(null)
    },
    [],
  )

  function handleOpenDir(name: string) {
    navigate(joinPath(path, name))
  }

  function handleOpenFile(entry: FsEntry) {
    setActiveEntry(entry)
    setPanel('view')
  }

  function handleRename(entry: FsEntry, newName: string) {
    const from = joinPath(path, entry.name)
    const to = joinPath(path, newName)
    rename.mutate({ from, to })
  }

  function handleDeleteRequest(entry: FsEntry) {
    setPendingDelete(entry)
  }

  function handleDeleteConfirm(recursive: boolean) {
    if (!pendingDelete) return
    const entryPath = joinPath(path, pendingDelete.name)
    deletePath.mutate({ path: entryPath, recursive })
    setPendingDelete(null)
    if (activeEntry?.name === pendingDelete.name) {
      setPanel('list')
      setActiveEntry(null)
    }
  }

  function handleMkdirSubmit(name: string) {
    const newPath = joinPath(path, name)
    mkdir.mutate(newPath, {
      onSuccess: () => setPanel('list'),
    })
  }

  // Container filesystem placeholder
  if (containerId) {
    return (
      <div
        className="flex flex-col items-start gap-3 p-5"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Container Filesystem
        </p>
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          Container filesystem browsing will be available with the Bind Mount Tracer
          (sub-project 7). For now, navigate to the host filesystem and use the
          bind mount tracer to locate this container's mounted paths.
        </p>
        <p className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          container: {containerId.slice(0, 12)}
        </p>
      </div>
    )
  }

  return (
    <div
      className="flex flex-col flex-1 min-h-0 gap-0"
      style={{ height: '100%' }}
    >
      {/* Toolbar */}
      <div
        className="flex items-center gap-3 px-4 py-2.5 shrink-0 flex-wrap"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <Breadcrumbs path={path} onNavigate={navigate} />

        <div className="flex items-center gap-2 ml-auto shrink-0">
          <button
            type="button"
            onClick={() => void refetch()}
            title="Refresh"
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              padding: '2px',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            <RefreshCw size={13} />
          </button>
          {isAdmin && (
            <>
              <button
                type="button"
                onClick={() =>
                  setPanel((p) => (p === 'mkdir' ? 'list' : 'mkdir'))
                }
                className="flex items-center gap-1 text-xs px-2 py-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <FolderPlus size={12} />
                New folder
              </button>
              <button
                type="button"
                onClick={() =>
                  setPanel((p) => (p === 'upload' ? 'list' : 'upload'))
                }
                className="flex items-center gap-1 text-xs px-2 py-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <UploadIcon size={12} />
                Upload
              </button>
            </>
          )}
        </div>
      </div>

      {/* Content area */}
      <div className="flex flex-col flex-1 min-h-0 overflow-auto p-4 gap-3">
        {/* Mkdir panel */}
        {panel === 'mkdir' && (
          <MkdirPanel
            onSubmit={handleMkdirSubmit}
            onCancel={() => setPanel('list')}
          />
        )}

        {/* Upload dropzone */}
        {panel === 'upload' && (
          <UploadDropzone
            nodeId={nodeId}
            destDir={path}
            onComplete={() => {
              void refetch()
              setPanel('list')
            }}
          />
        )}

        {/* File viewer */}
        {panel === 'view' && activeEntry && (
          <FileViewer
            nodeId={nodeId}
            dirPath={path}
            entry={activeEntry}
            onClose={() => { setPanel('list'); setActiveEntry(null) }}
            onEditRequest={isAdmin ? () => setPanel('edit') : undefined}
            isAdmin={!!isAdmin}
          />
        )}

        {/* File editor */}
        {panel === 'edit' && activeEntry && (
          <FileEditor
            nodeId={nodeId}
            dirPath={path}
            entry={activeEntry}
            onClose={() => { setPanel('view') }}
          />
        )}

        {/* Directory listing */}
        {(panel === 'list' || panel === 'mkdir' || panel === 'upload') && (
          <div
            className="flex flex-col flex-1 min-h-0"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            {isLoading && (
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
            {isError && (
              <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
                Failed to load directory. Check that the path exists and the
                node is reachable.
              </div>
            )}
            {data && (
              <FileList
                entries={data.entries}
                truncated={data.truncated}
                onOpenDir={handleOpenDir}
                onOpenFile={handleOpenFile}
                onRename={handleRename}
                onDelete={handleDeleteRequest}
                isAdmin={!!isAdmin}
              />
            )}
          </div>
        )}
      </div>

      {/* Delete confirmation modal */}
      {pendingDelete && (
        <DeleteConfirmModal
          entry={pendingDelete}
          onConfirm={handleDeleteConfirm}
          onCancel={() => setPendingDelete(null)}
        />
      )}
    </div>
  )
}
