import { apiPost } from '../api'

interface InstallScriptResponse {
  script: string
}

/**
 * Fetch the hardened agent install script for a node. Goes through apiPost so the
 * admin + TOTP step-up 428 flow (StepUp modal + retry) is handled. The returned
 * script embeds a single-use, short-lived enrollment token.
 */
export async function fetchAgentInstallScript(nodeId: string): Promise<string> {
  const res = await apiPost<InstallScriptResponse>(
    `/api/nodes/${encodeURIComponent(nodeId)}/agent/install`,
    {},
  )
  return res.script
}
