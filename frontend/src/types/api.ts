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

// Node types

export type NodeType = 'proxmox' | 'standalone' | 'ssh'
export type NodeStatus = 'ok' | 'unreachable' | 'error' | 'unknown'
export type ProxmoxAuthStatus = 'confirmed' | 'unauthed' | 'marker_only' | 'none'
export type CredentialMethod = 'ssh_key' | 'ssh_password'

export interface Capabilities {
  proxmox: boolean
  docker: boolean
  agent: boolean
  systemd: boolean
  cron: boolean
}

export interface NodeView {
  id: string
  name: string
  type: NodeType
  host: string
  port: number
  auth_method: string
  os_type: string
  capabilities: Capabilities
  proxmox_auth_status: ProxmoxAuthStatus
  status: NodeStatus
  last_error?: string
  last_seen?: string
  created_at: string
  updated_at: string
}

export interface NodeCredentials {
  method: CredentialMethod
  ssh_user: string
  ssh_password?: string
  ssh_private_key?: string
  ssh_passphrase?: string
  proxmox_token_id?: string
  proxmox_secret?: string
  docker_tls_ca?: string
  docker_tls_cert?: string
  docker_tls_key?: string
}

export interface CreateNodeRequest {
  name: string
  host: string
  ssh_port: number
  credentials: NodeCredentials
  proxmox_endpoint?: string
  proxmox_tls_insecure?: boolean
  docker_endpoint?: string
  pinned_host_key?: string
  ack_insecure_docker?: boolean
  accepted_host_key: string
  type_override?: NodeType
}

export interface RenameNodeRequest {
  name: string
}

export interface ProbePreviewRequest {
  host: string
  ssh_port: number
  credentials: NodeCredentials
  proxmox_endpoint?: string
  proxmox_tls_insecure?: boolean
  docker_endpoint?: string
  ack_insecure_docker?: boolean
}

export interface PreviewResult {
  type: NodeType
  os_type: string
  capabilities: Capabilities
  proxmox_auth_status: ProxmoxAuthStatus
  reachable_ssh: boolean
  ssh_host_key_sha256: string
  ssh_host_key_line: string
  docker_version?: string
  proxmox_version?: string
  probe_errors?: Record<string, string>
}

export interface NodesListResponse {
  nodes: NodeView[]
}

// Tree types

export type ContainerStatus = 'running' | 'exited' | 'paused' | 'restarting' | 'dead' | 'created'
export type VMKind = 'qemu' | 'lxc'

export interface VM {
  id: string
  node_id: string
  kind: VMKind
  proxmox_vmid: number
  proxmox_node: string
  name: string
  status: string
  os_type?: string
  stale: boolean
  last_seen: string
}

export interface Container {
  id: string
  node_id: string
  docker_id: string
  name: string
  image: string
  image_id?: string
  status: ContainerStatus
  compose_project?: string
  stale: boolean
  last_seen: string
}

export interface TreeNode {
  id: string
  name: string
  type: NodeType
  host: string
  status: NodeStatus
  capabilities: Capabilities
  proxmox_auth_status: ProxmoxAuthStatus
  seq: number
  vms: VM[]
  containers: Container[]
}

export interface TreeResponse {
  nodes: TreeNode[]
}

export type DeltaOp = 'added' | 'updated' | 'removed'
export type DeltaKind = 'vm' | 'container'

export interface Delta {
  op: DeltaOp
  kind: DeltaKind
  node_id: string
  seq: number
  vm?: VM
  container?: Container
}

export interface CycleMessage {
  node_id: string
  seq: number
  deltas: Delta[]
}

// Tree selection discriminated union

export type TreeSelection =
  | { kind: 'node'; nodeId: string }
  | { kind: 'vm'; nodeId: string; vmId: string; vmKind: VMKind }
  | { kind: 'container'; nodeId: string; containerId: string }
  | { kind: 'fs-root'; nodeId: string; containerId?: string }
