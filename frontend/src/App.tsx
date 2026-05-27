import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from './providers/ThemeProvider'
import { queryClient } from './lib/queryClient'
import { AuthGuard } from './components/AuthGuard'
import { useSetupStatus } from './hooks/useSetupStatus'
import Login from './pages/Login'
import Setup from './pages/Setup'
import Dashboard from './pages/Dashboard'
import Nodes from './pages/Nodes'
import Resources from './pages/Resources'
import Logs from './pages/Logs'
import Security from './pages/Security'

function SetupRedirect() {
  const navigate = useNavigate()
  const { data, isLoading } = useSetupStatus()

  useEffect(() => {
    if (!isLoading && data?.needs_setup) {
      navigate('/setup', { replace: true })
    }
  }, [data, isLoading, navigate])

  return null
}

function AppRoutes() {
  return (
    <>
      <SetupRedirect />
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/setup" element={<Setup />} />
        <Route
          path="/"
          element={
            <AuthGuard>
              <Dashboard />
            </AuthGuard>
          }
        />
        <Route
          path="/nodes"
          element={
            <AuthGuard>
              <Nodes />
            </AuthGuard>
          }
        />
        <Route
          path="/resources"
          element={
            <AuthGuard>
              <Resources />
            </AuthGuard>
          }
        />
        <Route
          path="/logs"
          element={
            <AuthGuard>
              <Logs />
            </AuthGuard>
          }
        />
        <Route
          path="/security"
          element={
            <AuthGuard>
              <Security />
            </AuthGuard>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
