import { create } from 'zustand'
import type { TreeNode, VM, Container, TreeSelection, Delta } from '../types/api'

interface NodeLiveState {
  lastSeq: number
}

interface TreeState {
  nodes: TreeNode[]
  liveState: Record<string, NodeLiveState>
  expanded: Set<string>
  selection: TreeSelection | null

  setNodes: (nodes: TreeNode[]) => void
  applyDelta: (nodeId: string, delta: Delta) => void
  setLastSeq: (nodeId: string, seq: number) => void
  getLastSeq: (nodeId: string) => number
  toggleExpanded: (key: string) => void
  setExpanded: (key: string, open: boolean) => void
  setSelected: (sel: TreeSelection | null) => void
}

function upsertVM(list: VM[], vm: VM): VM[] {
  const idx = list.findIndex((v) => v.id === vm.id)
  if (idx === -1) return [...list, vm]
  const next = [...list]
  next[idx] = vm
  return next
}

function upsertContainer(list: Container[], c: Container): Container[] {
  const idx = list.findIndex((v) => v.id === c.id)
  if (idx === -1) return [...list, c]
  const next = [...list]
  next[idx] = c
  return next
}

export const useTreeStore = create<TreeState>()((set, get) => ({
  nodes: [],
  liveState: {},
  expanded: new Set<string>(),
  selection: null,

  setNodes: (nodes) =>
    set({ nodes }),

  applyDelta: (nodeId, delta) =>
    set((state) => {
      const nodes = state.nodes.map((node) => {
        if (node.id !== nodeId) return node

        if (delta.kind === 'vm') {
          if (delta.op === 'removed') {
            return { ...node, vms: node.vms.filter((v) => v.id !== delta.vm?.id) }
          }
          if (delta.vm) {
            return { ...node, vms: upsertVM(node.vms, delta.vm) }
          }
        }

        if (delta.kind === 'container') {
          if (delta.op === 'removed') {
            return { ...node, containers: node.containers.filter((c) => c.id !== delta.container?.id) }
          }
          if (delta.container) {
            return { ...node, containers: upsertContainer(node.containers, delta.container) }
          }
        }

        return node
      })
      return { nodes }
    }),

  setLastSeq: (nodeId, seq) =>
    set((state) => ({
      liveState: { ...state.liveState, [nodeId]: { lastSeq: seq } },
    })),

  getLastSeq: (nodeId) => get().liveState[nodeId]?.lastSeq ?? 0,

  toggleExpanded: (key) =>
    set((state) => {
      const next = new Set(state.expanded)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return { expanded: next }
    }),

  setExpanded: (key, open) =>
    set((state) => {
      const next = new Set(state.expanded)
      if (open) {
        next.add(key)
      } else {
        next.delete(key)
      }
      return { expanded: next }
    }),

  setSelected: (sel) => set({ selection: sel }),
}))
