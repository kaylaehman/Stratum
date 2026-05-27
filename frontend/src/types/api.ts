export interface User {
  id: string
  username: string
  email?: string
  role: string
}

export interface LoginResponse {
  access_token: string
  expires_at: string
  user: User
}

export interface SetupStatusResponse {
  needs_setup: boolean
}

export interface SetupAdminRequest {
  username: string
  password: string
  email?: string
}

export interface SetupAdminResponse {
  id: string
  username: string
}

export interface HealthResponse {
  status: string
  db: boolean
  uptime_seconds: number
}

export interface RefreshResponse {
  access_token: string
  expires_at: string
}

export interface ApiError {
  error: string
}
