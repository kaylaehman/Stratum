import { AppShell } from '../components/layout/AppShell'
import { LogViewer } from '../components/logs/LogViewer'

export default function Logs() {
  return (
    <AppShell>
      <div className="flex flex-col flex-1 min-h-0 h-full w-full">
        <LogViewer />
      </div>
    </AppShell>
  )
}
