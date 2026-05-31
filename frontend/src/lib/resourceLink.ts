/**
 * Builds a canonical deep-link URL to the Resources page, optionally targeting
 * a specific node and/or container.
 *
 * Canonical shape:  /resources?node=<nodeId>&container=<containerId>
 *
 * Either param may be omitted:
 *  - resourceLink(nodeId)              → /resources?node=<nodeId>
 *  - resourceLink(undefined, ctrId)    → /resources?container=<ctrId>
 *  - resourceLink(nodeId, ctrId)       → /resources?node=<nodeId>&container=<ctrId>
 */
export function resourceLink(nodeId?: string, containerId?: string): string {
  const p = new URLSearchParams()
  if (nodeId) p.set('node', nodeId)
  if (containerId) p.set('container', containerId)
  return `/resources?${p.toString()}`
}
