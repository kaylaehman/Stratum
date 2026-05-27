import { useAuthStore } from '../store/auth'

const CHUNK_SIZE = 4 * 1024 * 1024 // 4 MB

export interface ChunkUploadOptions {
  nodeId: string
  file: File
  targetPath: string
  onProgress: (received: number, total: number) => void
  signal: AbortSignal
}

export interface UploadStatusResponse {
  received: number
}

export interface UploadChunkResponse {
  received: number
}

export interface UploadFinishResponse {
  path: string
  bytes: number
}

export class UploadAbortedError extends Error {
  constructor() {
    super('Upload cancelled')
    this.name = 'UploadAbortedError'
  }
}

export class FileExistsError extends Error {
  constructor() {
    super('file_exists')
    this.name = 'FileExistsError'
  }
}

export class UploadApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
  ) {
    super(`Upload API error ${status}: ${code}`)
    this.name = 'UploadApiError'
  }
}

function authHeaders(): Record<string, string> {
  const { accessToken } = useAuthStore.getState()
  return accessToken ? { Authorization: `Bearer ${accessToken}` } : {}
}

async function getUploadStatus(
  nodeId: string,
  targetPath: string,
  signal: AbortSignal,
): Promise<number> {
  const url = `/api/nodes/${nodeId}/fs/upload/status?path=${encodeURIComponent(targetPath)}`
  const res = await fetch(url, { headers: authHeaders(), signal })
  if (!res.ok) return 0
  const body = (await res.json()) as UploadStatusResponse
  return body.received ?? 0
}

async function sendChunk(
  nodeId: string,
  targetPath: string,
  chunk: Blob,
  offset: number,
  signal: AbortSignal,
): Promise<number> {
  const url =
    `/api/nodes/${nodeId}/fs/upload/chunk` +
    `?path=${encodeURIComponent(targetPath)}&offset=${offset}`

  const res = await fetch(url, {
    method: 'PUT',
    headers: {
      ...authHeaders(),
      'Content-Type': 'application/octet-stream',
    },
    body: chunk,
    signal,
  })

  if (res.status === 409) {
    const body = (await res.json()) as { error: string; received: number }
    if (body.error === 'offset_mismatch') return body.received
    throw new UploadApiError(409, body.error)
  }

  if (!res.ok) {
    let code = res.statusText
    try {
      const b = (await res.json()) as { error?: string }
      if (b.error) code = b.error
    } catch { /* ignore */ }
    throw new UploadApiError(res.status, code)
  }

  const body = (await res.json()) as UploadChunkResponse
  return body.received
}

async function finishUpload(
  nodeId: string,
  targetPath: string,
  overwrite: boolean,
  signal: AbortSignal,
): Promise<UploadFinishResponse> {
  const url =
    `/api/nodes/${nodeId}/fs/upload/finish` +
    `?path=${encodeURIComponent(targetPath)}&overwrite=${overwrite}`

  const res = await fetch(url, {
    method: 'POST',
    headers: authHeaders(),
    signal,
  })

  if (res.status === 409) {
    const body = (await res.json()) as { error: string }
    if (body.error === 'file_exists') throw new FileExistsError()
    throw new UploadApiError(409, body.error)
  }

  if (!res.ok) {
    let code = res.statusText
    try {
      const b = (await res.json()) as { error?: string }
      if (b.error) code = b.error
    } catch { /* ignore */ }
    throw new UploadApiError(res.status, code)
  }

  return res.json() as Promise<UploadFinishResponse>
}

export async function cancelUpload(
  nodeId: string,
  targetPath: string,
): Promise<void> {
  const url =
    `/api/nodes/${nodeId}/fs/upload/chunk?path=${encodeURIComponent(targetPath)}`
  await fetch(url, { method: 'DELETE', headers: authHeaders() })
}

const MAX_CHUNK_RETRIES = 3

async function uploadChunks(
  nodeId: string,
  file: File,
  targetPath: string,
  initialOffset: number,
  onProgress: (received: number, total: number) => void,
  signal: AbortSignal,
): Promise<void> {
  let offset = initialOffset

  while (offset < file.size) {
    if (signal.aborted) throw new UploadAbortedError()

    const chunk = file.slice(offset, offset + CHUNK_SIZE)
    let attempt = 0
    let received = offset

    while (attempt < MAX_CHUNK_RETRIES) {
      try {
        received = await sendChunk(nodeId, targetPath, chunk, offset, signal)
        break
      } catch (err) {
        if (signal.aborted) throw new UploadAbortedError()
        if (err instanceof UploadApiError) throw err
        attempt++
        if (attempt >= MAX_CHUNK_RETRIES) throw err
        await new Promise((r) => setTimeout(r, 500 * attempt))
      }
    }

    offset = received
    onProgress(offset, file.size)
  }
}

/**
 * Runs the full status → chunk-loop → finish upload sequence.
 * Pass `overwrite: true` to skip the FileExistsError on finish.
 * Throws FileExistsError when finish returns 409 file_exists.
 */
export async function runChunkedUpload(
  opts: ChunkUploadOptions,
  overwrite = false,
): Promise<UploadFinishResponse> {
  const { nodeId, file, targetPath, onProgress, signal } = opts

  if (signal.aborted) throw new UploadAbortedError()

  const received = await getUploadStatus(nodeId, targetPath, signal)
  onProgress(received, file.size)

  await uploadChunks(nodeId, file, targetPath, received, onProgress, signal)

  return finishUpload(nodeId, targetPath, overwrite, signal)
}
