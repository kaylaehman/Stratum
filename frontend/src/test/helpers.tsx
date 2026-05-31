/**
 * Shared test helpers: QueryClient wrapper and auth store seeding.
 */
import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import type { User } from '../types/api'

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
}

interface WrapperProps {
  children: React.ReactNode
  client?: QueryClient
}

export function Wrapper({ children, client }: WrapperProps) {
  const qc = client ?? makeQueryClient()
  return (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  )
}

/** Seed the zustand auth store so role-based hooks work without hitting /api/me. */
export function seedAuth(role: User['role'] = 'admin') {
  useAuthStore.setState({
    accessToken: 'test-token',
    user: {
      id: '1',
      username: 'testuser',
      email: 'test@example.com',
      role,
    },
  })
}

export function clearAuth() {
  useAuthStore.setState({ accessToken: null, user: null })
}
