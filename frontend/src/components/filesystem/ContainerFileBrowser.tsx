import { useState } from 'react'
import { Loader, X } from 'lucide-react'
import { Breadcrumbs } from './Breadcrumbs'
import { FileList } from './FileList'
import { useContainerDir, readContainerFile } from '../../lib/api/fs'
import { ApiError } from '../../lib/api'
import type { FsEntry } from '../../types/api'

function joinPath(dir: string, name: string): string {
  return dir === '/' ? `/${name}` : `${dir}/${name}`
}

function errorCode(err: unknown): string | undefined {
  return err instanceof ApiError ? (err.body as { error?: string })?.error : undefined
}

// ContainerFileBrowser is a read-only view of a container's filesystem (listing
// via docker exec, file read via the archive API). No mkdir/upload/edit/delete —
// container writes are out of scope.
export function ContainerFileBrowser({ containerId }: { containerId: string }) {
  const [path, setPath] = useState('/')
  const { data, isLoading, isError, error } = useContainerDir(containerId, path)
  const [file, setFile] = useState<{ name: string; content: string; tooLarge: boolean } | null>(null)
  const [fileErr, setFileErr] = useState('')

  const noShell = errorCode(error) === 'container_no_shell'

  function navigate(p: string) {
    setPath(p)
    setFile(null)
    setFileErr('')
  }

  async function openFile(entry: FsEntry) {
    setFileErr('')
    try {
      const res = await readContainerFile(containerId, joinPath(path, entry.name))
      setFile({ name: entry.name, content: res.content, tooLarge: res.tooLarge })
    } catch {
      setFileErr(`Couldn't read ${entry.name}.`)
    }
  }

  return (
    <div className="flex flex-col flex-1 min-h-0" style={{ height: '100%' }}>
      {/* Toolbar */}
      <div
        className="flex items-center gap-3 px-4 py-2.5 shrink-0 flex-wrap"
        style={{ backgroundColor: 'var(--bg-surface)', borderBottom: '1px solid var(--border-subtle)' }}
      >
        <Breadcrumbs path={path} onNavigate={navigate} />
        <span
          className="text-xs ml-auto px-1.5 py-0.5 font-mono"
          style={{
            color: 'var(--text-muted)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          read-only
        </span>
      </div>

      <div className="flex flex-col flex-1 min-h-0 overflow-auto p-4 gap-3">
        {isLoading && (
          <div className="flex items-center gap-2 px-1 py-6">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</span>
          </div>
        )}

        {noShell && (
          <div className="px-4 py-4 text-xs" style={{ color: 'var(--text-secondary)' }}>
            This container has no shell, so its filesystem can't be listed directly (common for
            distroless images). Use the host filesystem and the bind-mount tracer to reach this
            container's mounted paths.
          </div>
        )}
        {isError && !noShell && (
          <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
            Couldn't list this directory. The container may be stopped, or the path may not exist.
          </div>
        )}

        {data && (
          <div
            className="flex flex-col flex-1 min-h-0"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            <FileList
              entries={data.entries}
              truncated={data.truncated}
              onOpenDir={(name) => navigate(joinPath(path, name))}
              onOpenFile={openFile}
              onRename={() => {}}
              onDelete={() => {}}
              isAdmin={false}
            />
          </div>
        )}

        {fileErr && (
          <div className="px-1 text-xs" style={{ color: 'var(--status-error)' }}>{fileErr}</div>
        )}

        {/* Simple read-only file viewer */}
        {file && (
          <div
            className="flex flex-col"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
            }}
          >
            <div
              className="flex items-center gap-2 px-3 py-2"
              style={{ borderBottom: '1px solid var(--border-subtle)' }}
            >
              <span className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>{file.name}</span>
              <button
                type="button"
                onClick={() => setFile(null)}
                className="ml-auto"
                style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', display: 'flex' }}
                title="Close"
              >
                <X size={13} />
              </button>
            </div>
            {file.tooLarge ? (
              <p className="px-3 py-4 text-xs" style={{ color: 'var(--status-warn)' }}>
                File is too large to preview.
              </p>
            ) : (
              <pre
                className="px-3 py-3 text-xs"
                style={{
                  margin: 0,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  fontFamily: 'monospace',
                  color: 'var(--text-secondary)',
                  maxHeight: '50vh',
                  overflow: 'auto',
                }}
              >
                {file.content}
              </pre>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
