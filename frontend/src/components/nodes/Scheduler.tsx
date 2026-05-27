import { useState, useCallback } from 'react'
import { Clock, Calendar, Pencil, Save, Loader, Timer } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import { useSchedule, useSetCron } from '../../lib/api/scheduler'
import { useMe } from '../../hooks/useMe'
import { ApiError } from '../../lib/api'
import type { CronJob } from '../../types/api'

interface SchedulerProps {
  nodeId: string
}

interface CronEditorProps {
  user: string
  jobs: CronJob[]
  nodeId: string
  onClose: () => void
}

function CronEditor({ user, jobs, nodeId, onClose }: CronEditorProps) {
  const initialContent = jobs.map((j) => j.raw || `${j.schedule} ${j.command}`).join('\n')
  const [content, setContent] = useState(initialContent)
  const { mutate: setCron, isPending, error, reset } = useSetCron()

  const handleChange = useCallback((val: string) => setContent(val), [])

  const apiErr = error as ApiError | null
  const errCode = apiErr ? (apiErr.body as { error?: string })?.error : null
  const errorMsg =
    errCode === 'cron_write_failed'
      ? 'Cron write failed — check node SSH access'
      : errCode === 'valid_user_required'
        ? 'Invalid user'
        : apiErr
          ? 'Save failed'
          : null

  function handleSave() {
    reset()
    setCron(
      { nodeId, request: { user, content } },
      { onSuccess: onClose },
    )
  }

  return (
    <div
      className="flex flex-col gap-2 mt-2 p-3"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <p className="font-mono text-xs" style={{ color: 'var(--status-warn)' }}>
        Warning: saving replaces {user}&apos;s entire crontab.
      </p>
      <div
        style={{
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          overflow: 'hidden',
          minHeight: '80px',
          maxHeight: '240px',
          overflowY: 'auto',
        }}
      >
        <CodeMirror
          value={content}
          onChange={handleChange}
          editable={!isPending}
          basicSetup={{ lineNumbers: true, foldGutter: false }}
          theme="dark"
          style={{ fontSize: '12px', fontFamily: "'Space Mono', monospace" }}
        />
      </div>
      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={isPending}
          onClick={handleSave}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
          style={{
            background: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: isPending ? 'var(--text-muted)' : 'var(--accent)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={12} className="animate-spin" /> : <Save size={12} />}
          Save
        </button>
        <button
          type="button"
          disabled={isPending}
          onClick={onClose}
          className="font-mono text-xs px-2.5 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

interface CronUserGroupProps {
  user: string
  jobs: CronJob[]
  nodeId: string
}

function CronUserGroup({ user, jobs, nodeId }: CronUserGroupProps) {
  const [editing, setEditing] = useState(false)

  return (
    <div
      className="flex flex-col gap-1 py-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
          {user}
        </span>
        {!editing && (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="flex items-center gap-1 font-mono text-xs px-2 py-0.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Pencil size={11} />
            Edit crontab
          </button>
        )}
      </div>
      <div className="flex flex-col gap-0.5 pl-2">
        {jobs.map((job, i) => (
          <div key={i} className="flex flex-col gap-0.5">
            <span className="font-mono text-xs truncate" style={{ color: 'var(--text-muted)' }} title={job.schedule}>
              {job.human || job.schedule}
            </span>
            <span className="font-mono text-xs truncate" style={{ color: 'var(--text-primary)' }} title={job.command}>
              {job.command}
            </span>
          </div>
        ))}
      </div>
      {editing && (
        <CronEditor
          user={user}
          jobs={jobs}
          nodeId={nodeId}
          onClose={() => setEditing(false)}
        />
      )}
    </div>
  )
}

export function Scheduler({ nodeId }: SchedulerProps) {
  const { data: me } = useMe()
  const { data, isLoading, isError } = useSchedule(nodeId)

  if (me?.role !== 'admin') return null

  const sectionStyle: React.CSSProperties = {
    borderTop: '1px solid var(--border-subtle)',
    paddingTop: '8px',
    marginTop: '8px',
  }

  const labelStyle: React.CSSProperties = {
    fontSize: '12px',
    color: 'var(--text-muted)',
  }

  if (isLoading) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5">
          <Clock size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Scheduled Tasks
          </span>
        </div>
        <Loader size={12} className="animate-spin mt-2" style={{ color: 'var(--text-muted)' }} />
      </div>
    )
  }

  if (isError) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5">
          <Clock size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Scheduled Tasks
          </span>
        </div>
        <p className="font-mono text-xs mt-2" style={{ color: 'var(--status-error)' }}>
          Could not read schedule — node unreachable over SSH
        </p>
      </div>
    )
  }

  const cronJobs = data?.cron ?? []
  const timers = data?.timers ?? []

  if (cronJobs.length === 0 && timers.length === 0) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5 mb-2">
          <Clock size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Scheduled Tasks
          </span>
        </div>
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No cron jobs or timers found.
        </p>
      </div>
    )
  }

  // Group cron jobs by user
  const cronByUser = cronJobs.reduce<Record<string, CronJob[]>>((acc, job) => {
    if (!acc[job.user]) acc[job.user] = []
    acc[job.user].push(job)
    return acc
  }, {})

  return (
    <div style={sectionStyle}>
      {/* Cron section */}
      {cronJobs.length > 0 && (
        <>
          <div className="flex items-center gap-1.5 mb-1">
            <Clock size={12} style={{ color: 'var(--text-muted)' }} />
            <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
              Cron Jobs
            </span>
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {cronJobs.length}
            </span>
          </div>
          <div className="flex flex-col">
            {Object.entries(cronByUser).map(([user, jobs]) => (
              <CronUserGroup key={user} user={user} jobs={jobs} nodeId={nodeId} />
            ))}
          </div>
        </>
      )}

      {/* systemd timers section */}
      {timers.length > 0 && (
        <div style={{ marginTop: cronJobs.length > 0 ? '12px' : undefined }}>
          <div className="flex items-center gap-1.5 mb-1">
            <Timer size={12} style={{ color: 'var(--text-muted)' }} />
            <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
              systemd Timers
            </span>
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {timers.length}
            </span>
          </div>
          <p className="font-mono text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
            Read-only — timer editing not yet supported.
          </p>
          <div className="flex flex-col">
            {timers.map((t) => (
              <div
                key={t.unit}
                className="flex flex-col gap-0.5 py-2"
                style={{ borderBottom: '1px solid var(--border-subtle)' }}
              >
                <div className="flex items-center gap-1.5">
                  <Calendar size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
                  <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {t.unit}
                  </span>
                  {t.activates && (
                    <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                      → {t.activates}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-3 pl-4">
                  {t.next && (
                    <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                      next: {t.next}
                    </span>
                  )}
                  {t.last && (
                    <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                      last: {t.last}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
