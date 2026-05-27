import { useMemo, useRef, useState } from 'react'
import {
  Folder,
  FileText,
  Link,
  AlertTriangle,
  Trash2,
  PenLine,
} from 'lucide-react'
import type { FsEntry } from '../../types/api'

function humanSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatMtime(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

interface EntryIconProps {
  entry: FsEntry
}

function EntryIcon({ entry }: EntryIconProps) {
  if (entry.is_symlink) {
    return <Link size={13} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
  }
  if (entry.is_dir) {
    return <Folder size={13} style={{ color: 'var(--accent)', flexShrink: 0 }} />
  }
  const classes = entry.classes ?? []
  if (classes.includes('world_writable')) {
    return <AlertTriangle size={13} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
  }
  return <FileText size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
}

function entryNameColor(entry: FsEntry): string {
  const classes = entry.classes ?? []
  if (classes.includes('world_writable')) return 'var(--status-error)'
  if (classes.includes('setuid') || classes.includes('setgid')) return 'var(--status-warn)'
  if (entry.is_symlink) return 'var(--text-secondary)'
  if (entry.is_dir) return 'var(--accent)'
  return 'var(--text-primary)'
}

interface RenameInputProps {
  initial: string
  onCommit: (next: string) => void
  onCancel: () => void
}

function RenameInput({ initial, onCommit, onCancel }: RenameInputProps) {
  const [val, setVal] = useState(initial)
  return (
    <input
      autoFocus
      type="text"
      value={val}
      onChange={(e) => setVal(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') onCommit(val)
        if (e.key === 'Escape') onCancel()
      }}
      onBlur={() => onCancel()}
      className="font-mono text-xs px-1"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-primary)',
        borderRadius: '2px',
        outline: 'none',
        width: '180px',
      }}
    />
  )
}

interface FileRowProps {
  entry: FsEntry
  onOpenDir: (name: string) => void
  onOpenFile: (entry: FsEntry) => void
  onRename: (entry: FsEntry, newName: string) => void
  onDelete: (entry: FsEntry) => void
  isAdmin: boolean
}

function FileRow({
  entry,
  onOpenDir,
  onOpenFile,
  onRename,
  onDelete,
  isAdmin,
}: FileRowProps) {
  const [renaming, setRenaming] = useState(false)

  function handleClick() {
    if (renaming) return
    if (entry.is_dir) onOpenDir(entry.name)
    else onOpenFile(entry)
  }

  function commitRename(newName: string) {
    setRenaming(false)
    if (newName && newName !== entry.name) onRename(entry, newName)
  }

  const owner = entry.owner
    ? `${entry.owner} (${entry.uid})`
    : String(entry.uid)
  const group = entry.group
    ? `${entry.group} (${entry.gid})`
    : String(entry.gid)
  const ownerGroup = `${owner}:${group}`
  const classes = entry.classes ?? []

  return (
    <div
      className="flex items-center gap-3 px-3 py-1.5 group"
      style={{
        borderBottom: '1px solid var(--border-subtle)',
        minHeight: '30px',
        cursor: entry.is_dir || !entry.is_symlink ? 'pointer' : 'default',
      }}
      onClick={handleClick}
    >
      {/* Icon */}
      <EntryIcon entry={entry} />

      {/* Name */}
      <div className="flex-1 min-w-0 flex items-center gap-1.5">
        {renaming ? (
          <RenameInput
            initial={entry.name}
            onCommit={commitRename}
            onCancel={() => setRenaming(false)}
          />
        ) : (
          <span
            className="font-mono text-xs truncate"
            style={{
              color: entryNameColor(entry),
              fontStyle: entry.is_symlink ? 'italic' : undefined,
            }}
          >
            {entry.name}
            {entry.is_symlink && entry.link_target && (
              <span style={{ color: 'var(--text-muted)' }}>
                {' '}→ {entry.link_target}
              </span>
            )}
          </span>
        )}
        {/* Class badges */}
        {classes.includes('setuid') && (
          <span
            className="text-xs px-1"
            style={{
              background: 'rgba(240,160,32,0.15)',
              color: 'var(--status-warn)',
              borderRadius: '2px',
              fontSize: '10px',
            }}
          >
            suid
          </span>
        )}
        {classes.includes('setgid') && (
          <span
            className="text-xs px-1"
            style={{
              background: 'rgba(240,160,32,0.15)',
              color: 'var(--status-warn)',
              borderRadius: '2px',
              fontSize: '10px',
            }}
          >
            sgid
          </span>
        )}
      </div>

      {/* Perms */}
      <span
        className="font-mono text-xs shrink-0 hidden md:block"
        style={{ color: 'var(--text-muted)', width: '120px' }}
        title={`${entry.mode_rwx} (${entry.mode_octal})`}
      >
        {entry.mode_rwx}
      </span>

      {/* Size */}
      <span
        className="font-mono text-xs shrink-0 text-right hidden sm:block"
        style={{ color: 'var(--text-secondary)', width: '70px' }}
      >
        {entry.is_dir ? '--' : humanSize(entry.size)}
      </span>

      {/* Modified */}
      <span
        className="font-mono text-xs shrink-0 hidden lg:block"
        style={{ color: 'var(--text-muted)', width: '130px' }}
      >
        {formatMtime(entry.mod_time)}
      </span>

      {/* Owner */}
      <span
        className="font-mono text-xs shrink-0 truncate hidden xl:block"
        style={{ color: 'var(--text-muted)', width: '150px' }}
        title={ownerGroup}
      >
        {ownerGroup}
      </span>

      {/* Actions (admin only, visible on hover) */}
      {isAdmin && (
        <div
          className="flex items-center gap-1 opacity-0 group-hover:opacity-100 shrink-0"
          style={{ transition: 'opacity 0.1s' }}
          onClick={(e) => e.stopPropagation()}
        >
          <button
            type="button"
            title="Rename"
            onClick={() => setRenaming(true)}
            className="p-0.5 rounded"
            style={{ color: 'var(--text-muted)', background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <PenLine size={12} />
          </button>
          <button
            type="button"
            title="Delete"
            onClick={() => onDelete(entry)}
            className="p-0.5 rounded"
            style={{ color: 'var(--status-error)', background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <Trash2 size={12} />
          </button>
        </div>
      )}
    </div>
  )
}

// ---- Column header ----

function ColHeader({ label, width, hide }: { label: string; width: string; hide?: string }) {
  return (
    <span
      className={`font-mono text-xs shrink-0 ${hide ?? ''}`}
      style={{ color: 'var(--text-muted)', width }}
    >
      {label}
    </span>
  )
}

interface FileListProps {
  entries: FsEntry[]
  truncated: boolean
  onOpenDir: (name: string) => void
  onOpenFile: (entry: FsEntry) => void
  onRename: (entry: FsEntry, newName: string) => void
  onDelete: (entry: FsEntry) => void
  isAdmin: boolean
}

const ROW_HEIGHT = 30
const OVERSCAN = 5

export function FileList({
  entries,
  truncated,
  onOpenDir,
  onOpenFile,
  onRename,
  onDelete,
  isAdmin,
}: FileListProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [scrollTop, setScrollTop] = useState(0)
  const containerHeight = 500 // approximation; parent clips overflow

  const sorted = useMemo(
    () =>
      [...entries].sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1
        if (!a.is_dir && b.is_dir) return 1
        return a.name.localeCompare(b.name)
      }),
    [entries],
  )

  // Use full render if list is small; windowing for large lists
  const useWindowing = sorted.length > 200

  const visibleStart = useWindowing
    ? Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN)
    : 0
  const visibleEnd = useWindowing
    ? Math.min(
        sorted.length,
        Math.ceil((scrollTop + containerHeight) / ROW_HEIGHT) + OVERSCAN,
      )
    : sorted.length

  const visibleEntries = sorted.slice(visibleStart, visibleEnd)
  const topPad = visibleStart * ROW_HEIGHT
  const bottomPad = (sorted.length - visibleEnd) * ROW_HEIGHT

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Column headers */}
      <div
        className="flex items-center gap-3 px-3 py-1.5 shrink-0"
        style={{
          borderBottom: '1px solid var(--border-default)',
          backgroundColor: 'var(--bg-elevated)',
        }}
      >
        <span style={{ width: 13 }} />
        <span className="flex-1 font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Name
        </span>
        <ColHeader label="Permissions" width="120px" hide="hidden md:block" />
        <ColHeader label="Size" width="70px" hide="hidden sm:block" />
        <ColHeader label="Modified" width="130px" hide="hidden lg:block" />
        <ColHeader label="Owner:Group" width="150px" hide="hidden xl:block" />
        {isAdmin && <span style={{ width: '44px' }} />}
      </div>

      {/* Scrollable list */}
      <div
        ref={scrollRef}
        style={{ overflowY: 'auto', flex: 1 }}
        onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
      >
        {useWindowing && <div style={{ height: topPad }} />}
        {visibleEntries.map((entry) => (
          <FileRow
            key={entry.name}
            entry={entry}
            onOpenDir={onOpenDir}
            onOpenFile={onOpenFile}
            onRename={onRename}
            onDelete={onDelete}
            isAdmin={isAdmin}
          />
        ))}
        {useWindowing && <div style={{ height: bottomPad }} />}
      </div>

      {truncated && (
        <div
          className="px-3 py-2 text-xs shrink-0"
          style={{
            color: 'var(--status-warn)',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--bg-surface)',
          }}
        >
          Directory listing truncated — too many entries to display all.
        </div>
      )}
    </div>
  )
}
