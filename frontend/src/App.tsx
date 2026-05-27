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
import Activity from './pages/Activity'
import Volumes from './pages/Volumes'
import Metrics from './pages/Metrics'
import Network from './pages/Network'
import Dependencies from './pages/Dependencies'
import Bulk from './pages/Bulk'
import Notifications from './pages/Notifications'
import Updates from './pages/Updates'
import Templates from './pages/Templates'
import Secrets from './pages/Secrets'
import CVE from './pages/CVE'
import Scripts from './pages/Scripts'
import Backups from './pages/Backups'

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
        <Route
          path="/activity"
          element={
            <AuthGuard>
              <Activity />
            </AuthGuard>
          }
        />
        <Route
          path="/volumes"
          element={
            <AuthGuard>
              <Volumes />
            </AuthGuard>
          }
        />
        <Route
          path="/metrics"
          element={
            <AuthGuard>
              <Metrics />
            </AuthGuard>
          }
        />
        <Route
          path="/network"
          element={
            <AuthGuard>
              <Network />
            </AuthGuard>
          }
        />
        <Route
          path="/dependencies"
          element={
            <AuthGuard>
              <Dependencies />
            </AuthGuard>
          }
        />
        <Route
          path="/bulk"
          element={
            <AuthGuard>
              <Bulk />
            </AuthGuard>
          }
        />
        <Route
          path="/notifications"
          element={
            <AuthGuard>
              <Notifications />
            </AuthGuard>
          }
        />
        <Route
          path="/updates"
          element={
            <AuthGuard>
              <Updates />
            </AuthGuard>
          }
        />
        <Route
          path="/templates"
          element={
            <AuthGuard>
              <Templates />
            </AuthGuard>
          }
        />
        <Route
          path="/secrets"
          element={
            <AuthGuard>
              <Secrets />
            </AuthGuard>
          }
        />
        <Route
          path="/cve"
          element={
            <AuthGuard>
              <CVE />
            </AuthGuard>
          }
        />
        <Route
          path="/scripts"
          element={
            <AuthGuard>
              <Scripts />
            </AuthGuard>
          }
        />
        <Route
          path="/backups"
          element={
            <AuthGuard>
              <Backups />
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
