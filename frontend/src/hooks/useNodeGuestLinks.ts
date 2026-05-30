import { useMemo } from 'react'
import { useNodes } from '../lib/api/nodes'
import type { TreeNode, VM, NodeView } from '../types/api'

// ---- Proxmox-guest ↔ standalone-node correlation ----
//
// A machine can appear twice: as a Proxmox GUEST (a VM/LXC under the Proxmox
// node) and as a separate STANDALONE Docker node (where its containers live,
// since Stratum can't see inside a guest from the host). This correlation nests
// the standalone node's containers under the matching guest, hides the
// standalone node from the top level, and lets the guest's detail pane borrow
// the linked node's full host detail.
//
// Resolve each docker node's target VMID (tri-state linked_vmid):
//   >= 100 → that vmid; 0 → none; null/undefined → AUTO (match a guest by name).

/** Resolve the VMID a docker node claims to run as, or undefined for "none". */
export function resolveTargetVmid(
  node: TreeNode,
  linkedVmid: number | undefined,
  vms: VM[],
): number | undefined {
  if (linkedVmid === 0) return undefined // NONE: force-unlinked
  if (typeof linkedVmid === 'number' && linkedVmid >= 100) return linkedVmid // explicit
  // AUTO: match a Proxmox guest whose name equals this node's name (case-insensitive).
  const match = vms.find((vm) => vm.name.toLowerCase() === node.name.toLowerCase())
  return match?.proxmox_vmid
}

export interface Correlation {
  /** linkedNodeByVmid[vmid] = the standalone TreeNode that runs as that guest. */
  linkedNodeByVmid: Map<number, TreeNode>
  /** Node ids hidden from the top level because they nest under a present guest. */
  hiddenNodeIds: Set<string>
}

/**
 * Build the guest↔standalone-node correlation. `nodes` are the rendered tree
 * nodes; `linkedVmidByNode` carries each node's manual linked_vmid (from the
 * Nodes API). Only links to a PRESENT guest hide a node; a node pointing at a
 * missing guest stays visible at the top level so it is never lost. When two
 * nodes resolve to the same guest, the first (deterministic by tree order)
 * wins and the rest stay at the top level.
 */
export function buildCorrelation(
  nodes: TreeNode[],
  linkedVmidByNode: Map<string, number | undefined>,
): Correlation {
  // All present guest vmids across every proxmox node.
  const presentVmids = new Set<number>()
  for (const n of nodes) {
    for (const vm of n.vms) presentVmids.add(vm.proxmox_vmid)
  }
  // Flat list of all guests (for AUTO name matching).
  const allVms: VM[] = nodes.flatMap((n) => n.vms)

  const linkedNodeByVmid = new Map<number, TreeNode>()
  const hiddenNodeIds = new Set<string>()

  for (const node of nodes) {
    // Only docker nodes carry containers worth nesting.
    if (!node.capabilities.docker) continue
    const target = resolveTargetVmid(node, linkedVmidByNode.get(node.id), allVms)
    if (target === undefined) continue
    if (!presentVmids.has(target)) continue // guest gone → keep node at top level
    if (linkedNodeByVmid.has(target)) continue // first match wins; others stay visible
    linkedNodeByVmid.set(target, node)
    hiddenNodeIds.add(node.id)
  }

  return { linkedNodeByVmid, hiddenNodeIds }
}

/**
 * Shared hook that computes the guest↔node correlation for a given set of tree
 * nodes. The tree response (TreeNode) doesn't carry linked_vmid; the Nodes API
 * does — this joins on node id and runs {@link buildCorrelation}. Consumed by
 * both the resource tree (to nest/hide) and the detail pane (to resolve a
 * selected guest to its linked host).
 */
export function useNodeGuestLinks(nodes: TreeNode[]): Correlation {
  const { data: nodeViews } = useNodes()

  // Map node id → manual linked_vmid (undefined = AUTO). Recomputed when the
  // Nodes API data changes (e.g. after editing a node's guest link).
  const linkedVmidByNode = useMemo(() => {
    const m = new Map<string, number | undefined>()
    for (const nv of (nodeViews as NodeView[] | undefined) ?? []) {
      m.set(nv.id, nv.linked_vmid)
    }
    return m
  }, [nodeViews])

  return useMemo(
    () => buildCorrelation(nodes, linkedVmidByNode),
    [nodes, linkedVmidByNode],
  )
}
