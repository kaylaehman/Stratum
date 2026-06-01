import { useAuthStore } from '../../store/auth'

export type DRExportFormat = 'json' | 'yaml' | 'md'

const EXT: Record<DRExportFormat, string> = {
  json: 'json',
  yaml: 'yaml',
  md: 'md',
}

/**
 * Downloads the DR manifest from /api/dr-export?format=...
 * Uses the same authenticated blob-download pattern as exportActivityCsv.
 */
export async function downloadDRExport(format: DRExportFormat): Promise<void> {
  const url = `/api/dr-export?format=${encodeURIComponent(format)}`
  const token = useAuthStore.getState().accessToken

  const headers: Record<string, string> = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(url, { method: 'GET', headers })
  if (!res.ok) {
    throw new Error(`DR export failed: ${res.status}`)
  }

  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = `dr-manifest.${EXT[format]}`
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(objectUrl)
}
