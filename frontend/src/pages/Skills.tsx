import { useMemo, useState } from 'react'
import {
  Wrench,
  Loader,
  Search,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  X,
  AlertTriangle,
  Terminal,
  ShieldAlert,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useSkills, useSkill } from '../lib/api/skills'
import type { SkillSummary, SkillStep } from '../types/api'

// ---- Chip ----

function Chip({ label }: { label: string }) {
  return (
    <span
      className="font-mono px-1.5 py-0.5"
      style={{
        background: 'var(--accent-glow)',
        border: '1px solid var(--accent-dim)',
        color: 'var(--accent)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      {label}
    </span>
  )
}

function PortChip({ port }: { port: number }) {
  return (
    <span
      className="font-mono px-1.5 py-0.5"
      style={{
        background: 'rgba(74,82,104,0.2)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-muted)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      :{port}
    </span>
  )
}

// ---- Skill card (list item) ----

interface SkillCardProps {
  skill: SkillSummary
  selected: boolean
  onSelect: (id: string) => void
}

function SkillCard({ skill, selected, onSelect }: SkillCardProps) {
  return (
    <button
      type="button"
      onClick={() => onSelect(skill.id)}
      className="text-left w-full"
      style={{
        backgroundColor: selected ? 'var(--accent-glow)' : 'var(--bg-surface)',
        border: `1px solid ${selected ? 'var(--accent-dim)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
        padding: '12px 14px',
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
        cursor: 'pointer',
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <span
          className="text-sm font-medium"
          style={{ color: selected ? 'var(--accent)' : 'var(--text-primary)' }}
        >
          {skill.name}
        </span>
        <span
          className="font-mono shrink-0 px-1.5 py-0.5"
          style={{
            background: skill.issue_count > 0 ? 'rgba(240,160,32,0.12)' : 'rgba(74,82,104,0.2)',
            border: `1px solid ${skill.issue_count > 0 ? 'rgba(240,160,32,0.4)' : 'var(--border-default)'}`,
            color: skill.issue_count > 0 ? 'var(--status-warn)' : 'var(--text-muted)',
            borderRadius: '3px',
            fontSize: '12px',
          }}
        >
          {skill.issue_count} {skill.issue_count === 1 ? 'issue' : 'issues'}
        </span>
      </div>

      {skill.description && (
        <p className="text-sm" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {skill.description}
        </p>
      )}

      {(skill.image_patterns.length > 0 || skill.port_hints.length > 0) && (
        <div className="flex flex-wrap items-center gap-1">
          {skill.image_patterns.map((p) => (
            <Chip key={p} label={p} />
          ))}
          {skill.port_hints.map((p) => (
            <PortChip key={p} port={p} />
          ))}
        </div>
      )}
    </button>
  )
}

// ---- Category group ----

interface CategoryGroupProps {
  category: string
  skills: SkillSummary[]
  selectedId: string | null
  onSelect: (id: string) => void
}

function CategoryGroup({ category, skills, selectedId, onSelect }: CategoryGroupProps) {
  const [open, setOpen] = useState(true)

  return (
    <div className="mb-5">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 w-full px-0 py-2 mb-3"
        style={{
          background: 'none',
          border: 'none',
          borderBottom: '1px solid var(--border-subtle)',
          cursor: 'pointer',
        }}
      >
        {open ? (
          <ChevronDown size={13} style={{ color: 'var(--text-muted)' }} />
        ) : (
          <ChevronRight size={13} style={{ color: 'var(--text-muted)' }} />
        )}
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          {category}
        </span>
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          ({skills.length})
        </span>
      </button>
      {open && (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
            gap: '12px',
          }}
        >
          {skills.map((s) => (
            <SkillCard key={s.id} skill={s} selected={s.id === selectedId} onSelect={onSelect} />
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Fix step ----

function StepRow({ step }: { step: SkillStep }) {
  return (
    <div
      className="flex flex-col gap-1.5 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-start gap-2">
        <Terminal size={12} style={{ color: 'var(--text-muted)', flexShrink: 0, marginTop: '2px' }} />
        <span className="text-sm flex-1" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {step.description}
        </span>
        {step.requires_approval && (
          <span
            className="flex items-center gap-1 font-mono shrink-0 px-1.5 py-0.5"
            style={{
              background: 'rgba(232,64,64,0.12)',
              border: '1px solid rgba(232,64,64,0.35)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              fontSize: '12px',
            }}
            title="This step requires explicit approval before running"
          >
            <ShieldAlert size={10} />
            approval
          </span>
        )}
      </div>
      {step.command && (
        <pre
          className="font-mono text-xs px-2.5 py-2 m-0"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            color: 'var(--text-primary)',
            overflowX: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {step.command}
        </pre>
      )}
    </div>
  )
}

// ---- Detail panel ----

function SkillDetailPanel({ id, onClose }: { id: string; onClose: () => void }) {
  const { data: skill, isLoading } = useSkill(id, true)
  const [openIssue, setOpenIssue] = useState<string | null>(null)

  return (
    <div
      style={{
        position: 'sticky',
        top: '24px',
        alignSelf: 'flex-start',
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        display: 'flex',
        flexDirection: 'column',
        maxHeight: 'calc(100vh - 120px)',
      }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-2 px-4 py-3 shrink-0"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <Wrench size={13} style={{ color: 'var(--text-secondary)' }} />
        <span
          className="text-xs font-medium uppercase tracking-wider flex-1 truncate"
          style={{ color: 'var(--text-primary)' }}
        >
          {skill?.name ?? 'Skill'}
        </span>
        {skill?.version && (
          <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
            v{skill.version}
          </span>
        )}
        <button
          type="button"
          onClick={onClose}
          style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
          title="Close"
        >
          <X size={14} />
        </button>
      </div>

      <div className="flex-1 overflow-auto p-4 flex flex-col gap-4">
        {isLoading && (
          <div className="flex items-center gap-2 py-4">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading skill...</span>
          </div>
        )}

        {!isLoading && skill && (
          <>
            {skill.description && (
              <p className="text-sm" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
                {skill.description}
              </p>
            )}

            {/* Meta chips */}
            {(skill.image_patterns.length > 0 || skill.port_hints.length > 0) && (
              <div className="flex flex-col gap-2">
                {skill.image_patterns.length > 0 && (
                  <div className="flex flex-col gap-1.5">
                    <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                      Image Patterns
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {skill.image_patterns.map((p) => <Chip key={p} label={p} />)}
                    </div>
                  </div>
                )}
                {skill.port_hints.length > 0 && (
                  <div className="flex flex-col gap-1.5">
                    <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                      Port Hints
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {skill.port_hints.map((p) => <PortChip key={p} port={p} />)}
                    </div>
                  </div>
                )}
              </div>
            )}

            {skill.docs_url && (
              <a
                href={skill.docs_url}
                target="_blank"
                rel="noreferrer noopener"
                className="flex items-center gap-1.5 text-xs self-start px-2 py-1"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--accent)',
                  borderRadius: '3px',
                  textDecoration: 'none',
                }}
              >
                <ExternalLink size={11} />
                Documentation
              </a>
            )}

            {/* Common issues */}
            <div className="flex flex-col gap-2">
              <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                Common Issues ({skill.common_issues.length})
              </span>
              {skill.common_issues.length === 0 ? (
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  No common issues documented.
                </span>
              ) : (
                skill.common_issues.map((issue) => {
                  const isOpen = openIssue === issue.id
                  return (
                    <div
                      key={issue.id}
                      style={{
                        backgroundColor: 'var(--bg-surface)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: '3px',
                      }}
                    >
                      <button
                        type="button"
                        onClick={() => setOpenIssue(isOpen ? null : issue.id)}
                        className="flex items-center gap-2 w-full px-3 py-2 text-left"
                        style={{ background: 'none', border: 'none', cursor: 'pointer' }}
                      >
                        {isOpen ? (
                          <ChevronDown size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
                        ) : (
                          <ChevronRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
                        )}
                        <AlertTriangle size={12} style={{ color: 'var(--status-warn)', flexShrink: 0 }} />
                        <span className="text-sm font-medium flex-1" style={{ color: 'var(--text-primary)' }}>
                          {issue.name}
                        </span>
                        <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
                          {issue.steps.length} {issue.steps.length === 1 ? 'step' : 'steps'}
                        </span>
                      </button>

                      {isOpen && (
                        <div style={{ borderTop: '1px solid var(--border-subtle)' }}>
                          {/* Symptoms */}
                          {issue.symptoms.length > 0 && (
                            <div className="px-3 py-2.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                              <span className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
                                Symptoms
                              </span>
                              <ul className="flex flex-col gap-1" style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                                {issue.symptoms.map((s, i) => (
                                  <li
                                    key={i}
                                    className="text-sm flex items-start gap-1.5"
                                    style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}
                                  >
                                    <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>•</span>
                                    {s}
                                  </li>
                                ))}
                              </ul>
                            </div>
                          )}

                          {/* Fix steps */}
                          <div>
                            <div className="px-3 pt-2.5 pb-1">
                              <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                                Fix Steps
                              </span>
                            </div>
                            {issue.steps.length === 0 ? (
                              <div className="px-3 py-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                                No steps documented.
                              </div>
                            ) : (
                              issue.steps.map((step) => <StepRow key={step.id} step={step} />)
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  )
                })
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}

// ---- Main page ----

export default function Skills() {
  const { data, isLoading } = useSkills()
  const skills = useMemo(() => data?.skills ?? [], [data])

  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return skills
    return skills.filter((s) => {
      return (
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        s.image_patterns.some((p) => p.toLowerCase().includes(q))
      )
    })
  }, [skills, filter])

  // Group by category, sorted alphabetically.
  const grouped = useMemo(() => {
    const map = new Map<string, SkillSummary[]>()
    for (const s of filtered) {
      const cat = s.category || 'Uncategorized'
      const list = map.get(cat)
      if (list) list.push(s)
      else map.set(cat, [s])
    }
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b))
  }, [filtered])

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: selectedId ? '1200px' : '1000px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <Wrench size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Skills
          </h1>
          <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
            ({skills.length})
          </span>
          <div className="flex-1" />
          {/* Search */}
          <div
            className="flex items-center gap-2 px-2.5 py-1.5"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              width: '280px',
            }}
          >
            <Search size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter by name, description, image..."
              className="text-sm flex-1"
              style={{
                background: 'transparent',
                border: 'none',
                color: 'var(--text-primary)',
                outline: 'none',
                minWidth: 0,
              }}
            />
            {filter && (
              <button
                type="button"
                onClick={() => setFilter('')}
                style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: 0 }}
                title="Clear"
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading skills...</span>
          </div>
        )}

        {/* Body: list + optional detail panel */}
        {!isLoading && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: selectedId ? 'minmax(0, 1fr) 420px' : 'minmax(0, 1fr)',
              gap: '20px',
              alignItems: 'start',
            }}
          >
            {/* List column */}
            <div>
              {skills.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No skills available.
                </div>
              ) : grouped.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No skills match “{filter}”.
                </div>
              ) : (
                grouped.map(([category, items]) => (
                  <CategoryGroup
                    key={category}
                    category={category}
                    skills={items}
                    selectedId={selectedId}
                    onSelect={setSelectedId}
                  />
                ))
              )}
            </div>

            {/* Detail column */}
            {selectedId && (
              <SkillDetailPanel id={selectedId} onClose={() => setSelectedId(null)} />
            )}
          </div>
        )}
      </div>
    </AppShell>
  )
}
