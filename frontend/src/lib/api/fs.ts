import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../../store/auth'
import { apiFetch, requestStepUp } from '../api'
import type {
  FsDirResponse,
  FsFileResponse,
  FsMkdirRequest,
  FsRenameRequest,
  FsSearchResponse,
  FsUploadResponse,
} from '../../types/api'

// ---- Query keys ----

export function dirKey(nodeId: string, path: string) {
  return ['fs', 'dir', nodeId, path] as const
}

// ---- Directory listing ----

export function useDir(nodeId: string, path: string) {
  return useQuery({
    queryKey: dirKey(nodeId, path),
    queryFn: () =>
      apiFetch<FsDirResponse>(
        `/api/nodes/${nodeId}/fs?path=${encodeURIComponent(path)}`,
      ),
    enabled: Boolean(nodeId && path),
  })
}

// ---- Recursive subtree search (deep "include subfolders" mode) ----

export function useFsSearch(nodeId: string, root: string, query: string) {
  return useQuery({
    queryKey: ['fs', 'search', nodeId, root, query] as const,
    queryFn: () =>
      apiFetch<FsSearchResponse>(
        `/api/nodes/${nodeId}/fs/search?path=${encodeURIComponent(root)}&q=${encodeURIComponent(query)}`,
      ),
    // The caller passes empty root/query to disable; the walk is expensive so
    // don't refetch on focus and treat results as fresh for a short window.
    enabled: Boolean(nodeId && root && query),
    staleTime: 15_000,
    refetchOnWindowFocus: false,
  })
}

// ---- Container filesystem (read-only: docker exec listing + archive read) ----

export function containerDirKey(containerId: string, path: string) {
  return ['container-fs', containerId, path] as const
}

export function useContainerDir(containerId: string, path: string) {
  return useQuery({
    queryKey: containerDirKey(containerId, path),
    queryFn: () =>
      apiFetch<FsDirResponse>(
        `/api/containers/${encodeURIComponent(containerId)}/fs?path=${encodeURIComponent(path)}`,
      ),
    enabled: Boolean(containerId && path),
    retry: false,
  })
}

export async function readContainerFile(
  containerId: string,
  path: string,
): Promise<{ content: string; tooLarge: boolean }> {
  const body = await apiFetch<FsFileResponse>(
    `/api/containers/${encodeURIComponent(containerId)}/fs/file?path=${encodeURIComponent(path)}`,
  )
  return { content: body.content ?? '', tooLarge: body.too_large }
}

// ---- Read file (returns content + captured Last-Modified) ----

export interface ReadFileResult {
  content: string
  tooLarge: boolean
  lastModified: string | null
}

export async function readFile(
  nodeId: string,
  path: string,
): Promise<ReadFileResult> {
  const { accessToken } = useAuthStore.getState()
  const headers: Record<string, string> = {}
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`

  const res = await fetch(
    `/api/nodes/${nodeId}/fs/file?path=${encodeURIComponent(path)}`,
    { headers },
  )
  if (!res.ok) {
    throw new Error(`Failed to read file: ${res.status}`)
  }
  const body = (await res.json()) as FsFileResponse
  return {
    content: body.content ?? '',
    tooLarge: body.too_large,
    lastModified: res.headers.get('Last-Modified'),
  }
}

// ---- Write file ----

export class StaleWriteError extends Error {
  constructor() {
    super('stale')
    this.name = 'StaleWriteError'
  }
}

export async function writeFile(
  nodeId: string,
  path: string,
  content: string,
  lastModified: string | null,
): Promise<void> {
  const { accessToken } = useAuthStore.getState()
  const headers: Record<string, string> = {
    'Content-Type': 'text/plain; charset=utf-8',
  }
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
  if (lastModified) headers['If-Unmodified-Since'] = lastModified

  const reqUrl = `/api/nodes/${nodeId}/fs/file?path=${encodeURIComponent(path)}`
  const reqInit: RequestInit = { method: 'PUT', headers, body: content }

  let res = await fetch(reqUrl, reqInit)

  // Step-up 2FA: retry once after a successful challenge
  if (res.status === 428) {
    let body428: unknown
    try { body428 = await res.json() } catch { body428 = {} }
    if ((body428 as { error?: string }).error === '2fa_required') {
      await requestStepUp()
      // Rebuild auth headers in case token refreshed during challenge
      const { accessToken: tok } = useAuthStore.getState()
      const retryHeaders: Record<string, string> = { ...headers }
      if (tok) retryHeaders['Authorization'] = `Bearer ${tok}`
      res = await fetch(reqUrl, { ...reqInit, headers: retryHeaders })
    }
  }

  if (res.status === 412) throw new StaleWriteError()
  if (!res.ok) {
    let msg = res.statusText
    try {
      const b = (await res.json()) as { error?: string }
      if (b.error) msg = b.error
    } catch {
      // ignore
    }
    throw new Error(msg)
  }
}

// ---- Download (triggers browser download) ----

export function downloadFile(nodeId: string, path: string): void {
  const { accessToken } = useAuthStore.getState()
  const url = `/api/nodes/${nodeId}/fs/download?path=${encodeURIComponent(path)}`
  // Use a temporary anchor with the token in Authorization header via a fetch+blob approach
  const name = path.split('/').pop() ?? 'download'
  void (async () => {
    const headers: Record<string, string> = {}
    if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
    const res = await fetch(url, { headers })
    if (!res.ok) return
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = name
    a.click()
    URL.revokeObjectURL(objectUrl)
  })()
}

// ---- Upload file (XHR for progress) ----

export interface UploadProgress {
  loaded: number
  total: number
}

export function uploadFile(
  nodeId: string,
  destDir: string,
  file: File,
  onProgress?: (p: UploadProgress) => void,
): Promise<FsUploadResponse> {
  return new Promise((resolve, reject) => {
    const { accessToken } = useAuthStore.getState()
    const fd = new FormData()
    fd.append('file', file)

    const xhr = new XMLHttpRequest()
    xhr.open(
      'POST',
      `/api/nodes/${nodeId}/fs/upload?path=${encodeURIComponent(destDir)}`,
    )
    if (accessToken) xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress({ loaded: e.loaded, total: e.total })
      }
    }
    xhr.onload = () => {
      if (xhr.status === 201) {
        resolve(JSON.parse(xhr.responseText) as FsUploadResponse)
      } else {
        reject(new Error(`Upload failed: ${xhr.status}`))
      }
    }
    xhr.onerror = () => reject(new Error('Upload network error'))
    xhr.send(fd)
  })
}

// ---- Mkdir mutation ----

export function useMkdir(nodeId: string, currentPath: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (path: string) =>
      apiFetch<void>(`/api/nodes/${nodeId}/fs/mkdir`, {
        method: 'POST',
        body: JSON.stringify({ path } satisfies FsMkdirRequest),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: dirKey(nodeId, currentPath) })
    },
  })
}

// ---- Rename mutation ----

export function useRename(nodeId: string, currentPath: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ from, to }: { from: string; to: string }) =>
      apiFetch<void>(`/api/nodes/${nodeId}/fs/rename`, {
        method: 'POST',
        body: JSON.stringify({ from, to } satisfies FsRenameRequest),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: dirKey(nodeId, currentPath) })
    },
  })
}

// ---- Delete mutation ----

export function useDeletePath(nodeId: string, currentPath: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      path,
      recursive,
    }: {
      path: string
      recursive: boolean
    }) =>
      apiFetch<void>(
        `/api/nodes/${nodeId}/fs?path=${encodeURIComponent(path)}&recursive=${String(recursive)}&confirm=yes`,
        { method: 'DELETE' },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: dirKey(nodeId, currentPath) })
    },
  })
}
