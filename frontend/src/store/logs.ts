import { create } from 'zustand'
import type { LogLine } from '../types/api'

export interface SelectedContainer {
  uuid: string
  dockerId: string
  name: string
}

export interface RichLogLine extends LogLine {
  /** Detected log level (from JSON parsing). */
  level?: string
}

interface FilterState {
  query: string
  isRegex: boolean
  levels: Set<string>
}

/** Fixed palette of per-container accent colors (left bar + label). */
const COLOR_PALETTE = [
  '#00c2cc', // accent teal
  '#4a9eff', // info blue
  '#22c97a', // green
  '#f0a020', // amber
  '#a78bfa', // violet
  '#f472b6', // pink
  '#34d399', // emerald
  '#fb923c', // orange
  '#60a5fa', // sky
  '#facc15', // yellow
]

const MAX_LINES = 10_000

/** Try to extract a log level from a JSON log line. */
function detectLevel(text: string): string | undefined {
  try {
    const obj = JSON.parse(text) as Record<string, unknown>
    const raw =
      obj['level'] ??
      obj['lvl'] ??
      obj['severity'] ??
      obj['Level'] ??
      obj['Lvl'] ??
      obj['Severity']
    if (typeof raw === 'string') return raw.toLowerCase()
    if (typeof raw === 'number') {
      // common numeric level conventions (e.g. pino: 10=trace, 20=debug, 30=info, 40=warn, 50=error)
      if (raw >= 50) return 'error'
      if (raw >= 40) return 'warn'
      if (raw >= 30) return 'info'
      if (raw >= 20) return 'debug'
      return 'trace'
    }
  } catch {
    // not JSON
  }
  return undefined
}

interface LogsState {
  selectedContainers: SelectedContainer[]
  /** Ring buffer; capped at MAX_LINES */
  lines: RichLogLine[]
  /** Total lines received (used to detect gaps) */
  totalReceived: number
  filter: FilterState
  paused: boolean
  /** Detected level values (for the level filter UI) */
  seenLevels: Set<string>
  /** Map docker_id -> color string */
  colorMap: Record<string, string>

  addContainer: (c: SelectedContainer) => void
  removeContainer: (dockerId: string) => void
  addLine: (line: LogLine) => void
  setFilterQuery: (query: string) => void
  setFilterRegex: (isRegex: boolean) => void
  toggleFilterLevel: (level: string) => void
  setFilterLevels: (levels: Set<string>) => void
  togglePause: () => void
  clear: () => void
}

function assignColor(colorMap: Record<string, string>, dockerId: string): Record<string, string> {
  if (colorMap[dockerId]) return colorMap
  const idx = Object.keys(colorMap).length % COLOR_PALETTE.length
  return { ...colorMap, [dockerId]: COLOR_PALETTE[idx] }
}

export const useLogsStore = create<LogsState>()((set, get) => ({
  selectedContainers: [],
  lines: [],
  totalReceived: 0,
  filter: { query: '', isRegex: false, levels: new Set() },
  paused: false,
  seenLevels: new Set(),
  colorMap: {},

  addContainer: (c) =>
    set((state) => {
      if (state.selectedContainers.some((x) => x.uuid === c.uuid)) return state
      return {
        selectedContainers: [...state.selectedContainers, c],
        colorMap: assignColor(state.colorMap, c.dockerId),
      }
    }),

  removeContainer: (dockerId) =>
    set((state) => ({
      selectedContainers: state.selectedContainers.filter((x) => x.dockerId !== dockerId),
    })),

  addLine: (line) => {
    if (get().paused) {
      // Still buffer into ring (but don't re-render filtered view)
      set((state) => {
        const level = detectLevel(line.text)
        const rich: RichLogLine = { ...line, level }
        const next = state.lines.length >= MAX_LINES
          ? [...state.lines.slice(1), rich]
          : [...state.lines, rich]
        const seenLevels = level
          ? new Set([...state.seenLevels, level])
          : state.seenLevels
        return { lines: next, totalReceived: state.totalReceived + 1, seenLevels }
      })
      return
    }
    set((state) => {
      const level = detectLevel(line.text)
      const rich: RichLogLine = { ...line, level }
      const next = state.lines.length >= MAX_LINES
        ? [...state.lines.slice(1), rich]
        : [...state.lines, rich]
      const seenLevels = level
        ? new Set([...state.seenLevels, level])
        : state.seenLevels
      return { lines: next, totalReceived: state.totalReceived + 1, seenLevels }
    })
  },

  setFilterQuery: (query) =>
    set((state) => ({ filter: { ...state.filter, query } })),

  setFilterRegex: (isRegex) =>
    set((state) => ({ filter: { ...state.filter, isRegex } })),

  toggleFilterLevel: (level) =>
    set((state) => {
      const levels = new Set(state.filter.levels)
      if (levels.has(level)) {
        levels.delete(level)
      } else {
        levels.add(level)
      }
      return { filter: { ...state.filter, levels } }
    }),

  setFilterLevels: (levels) =>
    set((state) => ({ filter: { ...state.filter, levels } })),

  togglePause: () => set((state) => ({ paused: !state.paused })),

  clear: () => set({ lines: [], totalReceived: 0, seenLevels: new Set() }),
}))

/** Color palette accessor (for use outside the store) */
export { COLOR_PALETTE }
