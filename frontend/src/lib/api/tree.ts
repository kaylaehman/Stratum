import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { apiGet } from '../api'
import { wsManager } from '../ws'
import { useTreeStore } from '../../store/tree'
import type { TreeResponse, CycleMessage, Delta } from '../../types/api'

export const TREE_KEY = ['tree'] as const

export function useTree() {
  return useQuery({
    queryKey: TREE_KEY,
    queryFn: () => apiGet<TreeResponse>('/api/tree'),
    staleTime: 30_000,
  })
}

function applyDeltas(nodeId: string, deltas: Delta[]): void {
  const store = useTreeStore.getState()
  for (const delta of deltas) {
    store.applyDelta(nodeId, delta)
  }
}

export function useTreeLiveUpdates(): void {
  const qc = useQueryClient()
  const { data } = useTree()
  const subscribedRef = useRef<Set<string>>(new Set())

  // Sync React Query data into Zustand whenever it loads/refreshes
  useEffect(() => {
    if (!data) return
    useTreeStore.getState().setNodes(data.nodes)
    for (const node of data.nodes) {
      useTreeStore.getState().setLastSeq(node.id, node.seq)
    }
  }, [data])

  useEffect(() => {
    if (!data) return

    const nodeIds = data.nodes.map((n) => n.id)

    function subscribeAll(): void {
      for (const nodeId of nodeIds) {
        if (!subscribedRef.current.has(nodeId)) {
          wsManager.send(JSON.stringify({ subscribe: `tree:${nodeId}` }))
          subscribedRef.current.add(nodeId)
        }
      }
    }

    // Connect and subscribe immediately (send is no-op if not open yet)
    wsManager.connect()
    subscribeAll()

    // Re-subscribe every time the socket (re)opens
    const unsubOpen = wsManager.onOpen(() => {
      subscribedRef.current.clear()
      subscribeAll()
    })

    const unsubscribers: Array<() => void> = []

    for (const nodeId of nodeIds) {
      const unsub = wsManager.subscribe(`tree:${nodeId}`, (raw) => {
        const msg = raw as CycleMessage
        if (!msg || typeof msg.seq !== 'number') return

        const store = useTreeStore.getState()
        const lastSeq = store.getLastSeq(nodeId)

        if (lastSeq === 0 || msg.seq === lastSeq + 1) {
          // In-sequence: apply deltas and advance seq
          if (msg.deltas && msg.deltas.length > 0) {
            applyDeltas(nodeId, msg.deltas)
          }
          store.setLastSeq(nodeId, msg.seq)
        } else if (msg.seq > lastSeq + 1) {
          // Gap detected: full refetch to re-baseline, reset subscriptions
          subscribedRef.current.clear()
          void qc.invalidateQueries({ queryKey: TREE_KEY })
        }
        // msg.seq <= lastSeq: stale/dup, silently ignore
      })
      unsubscribers.push(unsub)
    }

    return () => {
      unsubOpen()
      for (const unsub of unsubscribers) {
        unsub()
      }
    }
  }, [data, qc])
}
