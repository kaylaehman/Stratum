import { useState } from 'react'
import type { UptimeMonitor, UptimeMonitorRequest } from '../../lib/api/uptime'

interface MonitorFormProps {
  initial?: UptimeMonitor
  onSubmit: (data: UptimeMonitorRequest) => void
  onCancel: () => void
  isPending: boolean
}

const defaultForm: UptimeMonitorRequest = {
  name: '',
  type: 'http',
  target: '',
  interval_seconds: 60,
  timeout_ms: 5000,
  expected: '',
  enabled: true,
}

export function MonitorForm({ initial, onSubmit, onCancel, isPending }: MonitorFormProps) {
  const [form, setForm] = useState<UptimeMonitorRequest>(
    initial
      ? {
          name: initial.name,
          type: initial.type,
          target: initial.target,
          interval_seconds: initial.interval_seconds,
          timeout_ms: initial.timeout_ms,
          expected: initial.expected,
          enabled: initial.enabled,
          node_id: initial.node_id,
        }
      : defaultForm,
  )

  function set<K extends keyof UptimeMonitorRequest>(k: K, v: UptimeMonitorRequest[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(form)
      }}
      className="flex flex-col gap-3"
    >
      <div className="flex flex-col gap-1">
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Name
        </label>
        <input
          className="px-2 py-1.5 rounded text-sm"
          style={{
            background: 'var(--bg-input)',
            border: '1px solid var(--border-subtle)',
            color: 'var(--text-primary)',
            outline: 'none',
          }}
          value={form.name}
          onChange={(e) => set('name', e.target.value)}
          required
          placeholder="My Service"
        />
      </div>

      <div className="flex gap-2">
        <div className="flex flex-col gap-1 flex-1">
          <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            Type
          </label>
          <select
            className="px-2 py-1.5 rounded text-sm"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
            value={form.type}
            onChange={(e) => set('type', e.target.value as UptimeMonitorRequest['type'])}
          >
            <option value="http">HTTP</option>
            <option value="tcp">TCP</option>
            <option value="icmp">ICMP / Ping</option>
          </select>
        </div>

        <div className="flex flex-col gap-1" style={{ width: '100px' }}>
          <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            Interval (s)
          </label>
          <input
            type="number"
            min={10}
            className="px-2 py-1.5 rounded text-sm"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
            value={form.interval_seconds}
            onChange={(e) => set('interval_seconds', Number(e.target.value))}
          />
        </div>

        <div className="flex flex-col gap-1" style={{ width: '110px' }}>
          <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            Timeout (ms)
          </label>
          <input
            type="number"
            min={100}
            className="px-2 py-1.5 rounded text-sm"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
            value={form.timeout_ms}
            onChange={(e) => set('timeout_ms', Number(e.target.value))}
          />
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Target
          {form.type === 'http' && (
            <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}> (URL)</span>
          )}
          {form.type === 'tcp' && (
            <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}> (host:port)</span>
          )}
          {form.type === 'icmp' && (
            <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}> (hostname or IP)</span>
          )}
        </label>
        <input
          className="px-2 py-1.5 rounded text-sm font-mono"
          style={{
            background: 'var(--bg-input)',
            border: '1px solid var(--border-subtle)',
            color: 'var(--text-primary)',
          }}
          value={form.target}
          onChange={(e) => set('target', e.target.value)}
          required
          placeholder={
            form.type === 'http'
              ? 'https://example.com/health'
              : form.type === 'tcp'
                ? '192.168.1.1:80'
                : '192.168.1.1'
          }
        />
      </div>

      {form.type === 'http' && (
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            Expected (optional)
            <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>
              {' '}— status code (e.g. 200) or body keyword
            </span>
          </label>
          <input
            className="px-2 py-1.5 rounded text-sm font-mono"
            style={{
              background: 'var(--bg-input)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
            value={form.expected}
            onChange={(e) => set('expected', e.target.value)}
            placeholder='200 or "healthy"'
          />
        </div>
      )}

      <div className="flex items-center gap-2 mt-1">
        <input
          type="checkbox"
          id="monitor-enabled"
          checked={form.enabled}
          onChange={(e) => set('enabled', e.target.checked)}
        />
        <label
          htmlFor="monitor-enabled"
          className="text-sm"
          style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}
        >
          Enabled
        </label>
      </div>

      <div className="flex gap-2 mt-2 justify-end">
        <button
          type="button"
          onClick={onCancel}
          className="px-3 py-1.5 rounded text-sm"
          style={{
            background: 'var(--bg-input)',
            border: '1px solid var(--border-subtle)',
            color: 'var(--text-secondary)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={isPending}
          className="px-3 py-1.5 rounded text-sm"
          style={{
            background: 'var(--accent)',
            border: 'none',
            color: '#fff',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.7 : 1,
          }}
        >
          {isPending ? 'Saving…' : initial ? 'Update' : 'Create'}
        </button>
      </div>
    </form>
  )
}
