export type UserRole = 'viewer' | 'operator' | 'admin'

export interface User {
  id: string
  username: string
  email?: string
  role: UserRole
  created_at?: string
}

// RBAC / User management types (Feature 30)

export interface UsersListResponse {
  users: User[]
}

export interface CreateUserRequest {
  username: string
  password: string
  email?: string
  role: UserRole
}

export interface UpdateRoleRequest {
  role: UserRole
}

export interface SessionView {
  id: string
  user_agent?: string
  ip?: string
  created_at: string
  expires_at: string
  current: boolean
  active: boolean
}

export interface SessionsListResponse {
  sessions: SessionView[]
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
  docker_endpoint?: string
  /**
   * Manual Proxmox-guest correlation (tri-state):
   *   undefined → AUTO (match a guest by name)
   *   0         → NONE (force-unlinked; never nest)
   *   >= 100    → explicit Proxmox VMID this node runs as
   */
  linked_vmid?: number
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

/**
 * PUT /api/nodes/{id} — partial node update. All fields optional; only the
 * provided fields are changed. Used for both rename (name) and Docker config
 * (docker_endpoint + docker TLS creds). Sending docker_endpoint as an empty
 * string clears the configured endpoint.
 */
export interface UpdateNodeRequest {
  name?: string
  docker_endpoint?: string
  credentials?: Pick<
    NodeCredentials,
    'docker_tls_ca' | 'docker_tls_cert' | 'docker_tls_key'
  >
  ack_insecure_docker?: boolean
  /**
   * Manual Proxmox-guest link, tri-state. Omit the key to leave unchanged;
   * send null for AUTO (match by name), 0 for NONE (force-unlinked), or a
   * Proxmox VMID (>= 100) to link explicitly.
   */
  linked_vmid?: number | null
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
  probe_hints?: Record<string, string>
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

// Filesystem types

export type FsEntryClass =
  | 'dir'
  | 'symlink'
  | 'world_writable'
  | 'setuid'
  | 'setgid'
  | 'sticky'
  | 'exec'

export interface FsEntry {
  name: string
  is_dir: boolean
  is_symlink: boolean
  link_target?: string
  size: number
  mod_time: string
  mode_octal: string
  mode_rwx: string
  uid: number
  gid: number
  owner?: string
  group?: string
  classes?: FsEntryClass[]
}

export interface FsDirResponse {
  entries: FsEntry[]
  truncated: boolean
}

export interface FsFileResponse {
  too_large: boolean
  content?: string
}

export interface FsMkdirRequest {
  path: string
}

export interface FsRenameRequest {
  from: string
  to: string
}

export interface FsUploadResponse {
  path: string
  bytes: number
}

// Chunked / resumable upload types (Feature F10)

export interface FsUploadStatusResponse {
  received: number
}

export interface FsUploadChunkResponse {
  received: number
}

export interface FsUploadFinishResponse {
  path: string
  bytes: number
}

// UID/GID Conflict Visualizer types

export type UidRowClass = 'match' | 'mismatch' | 'unresolvable'

export interface UidRow {
  id: number
  host_name?: string
  container_name?: string
  on_host: boolean
  on_container: boolean
  class: UidRowClass
}

export interface UidAnalysis {
  uid_rows: UidRow[]
  gid_rows: UidRow[]
  legend: {
    match: number
    mismatch: number
    unresolvable: number
  }
}

export interface ContainerMount {
  type: string
  source: string
  destination: string
  mode: string
  rw: boolean
}

export interface ContainerInspect {
  id: string
  name: string
  image: string
  image_id?: string
  state: string
  config_user?: string
  mounts: ContainerMount[]
  privileged: boolean
  cap_add?: string[]
  cap_drop?: string[]
  pid_mode?: string
  network_mode?: string
  repo_digests?: string[]
  run_uid: number
  run_gid: number
  supplementary_gids?: number[]
  is_root: boolean
}

export interface FileVerdict {
  file_uid: number
  file_gid: number
  file_mode_octal: string
  host_owner_name?: string
  host_group_name?: string
  container_owner_name?: string
  eff_uid: number
  eff_gid: number
  supplementary_gids?: number[]
  process_is_root: boolean
  root_override: boolean
  category: 'owner' | 'owner(root)' | 'group' | 'other'
  read_granted: boolean
  write_granted: boolean
  exec_granted: boolean
  reason: string
}

// Diagnostic ("Why Is This Broken?") types

export type DiagnosticStepStatus = 'ok' | 'warn' | 'bad'

export interface DiagnosticStep {
  kind: string
  label: string
  detail: string
  status: DiagnosticStepStatus
}

export interface SuggestedFix {
  command: string
  rationale: string
  warning?: string
}

export interface DiagnosticBindMount {
  exposed: boolean
  container_path?: string
  rw: boolean
  via_source?: string
  via_destination?: string
  is_named_volume: boolean
  volume_name?: string
}

export interface DiagnosticVerdict {
  access_granted: boolean
  summary: string
}

export interface DiagnosticAcl {
  available: boolean
  entries?: string[]
}

export interface DiagnosticEffectiveAccess {
  read: boolean
  write: boolean
  exec: boolean
  decided_by: string
  category: string
  confidence: 'high' | 'medium' | 'low'
}

export interface DiagnosticResult {
  host_path: string
  file_uid: number
  file_gid: number
  file_mode: string
  run_uid: number
  run_gid: number
  is_root: boolean
  bind_mount: DiagnosticBindMount
  verdict: DiagnosticVerdict
  acl: DiagnosticAcl
  effective_access: DiagnosticEffectiveAccess
  steps: DiagnosticStep[]
  fixes: SuggestedFix[]
}

// Bind Mount Tracer types

export interface MountView {
  container_id: string
  type: 'bind' | 'volume' | string
  source: string
  destination: string
  volume_name?: string
  rw: boolean
  shared: boolean
  traceable: boolean
}

export interface MountsResponse {
  mounts: MountView[]
}

export type MountRelation = 'equal' | 'a_parent_of_b' | 'b_parent_of_a'

export interface ReverseHit {
  container_id: string
  source: string
  destination: string
  rw: boolean
  relation: MountRelation
}

export interface ReverseMountsResponse {
  containers: ReverseHit[]
}

export interface SharedEntry {
  kind: 'bind' | 'volume'
  key: string
  container_ids: string[]
}

export interface SharedMountsResponse {
  shared: SharedEntry[]
}

// Log streaming types

export type LogStream = 'stdout' | 'stderr'

/** A single log line pushed over the WebSocket by the server. */
export interface LogLine {
  /** UUID of the container (Docker ID, as sent by server after subscribe). */
  container_id: string
  /** RFC3339Nano UTC timestamp; may be empty string. */
  ts: string
  stream: LogStream
  text: string
  truncated?: boolean
}

/** Hello message sent by server on ws connect. */
export interface WsHello {
  client_id: string
}

export interface LogSubscribeRequest {
  container_id: string
  ws_client_id: string
}

export interface LogSubscribeResponse {
  topic: string
}

// Security types (Sub-project 8)

export type FlagType =
  | 'privileged'
  | 'cap'
  | 'seccomp'
  | 'apparmor'
  | 'device'
  | 'userns_host'
  | 'pid_host'
  | 'net_host'
  | 'root'

export interface SecurityFlag {
  type: FlagType
  key: string
  risk: string
  acknowledged: boolean
}

export interface FlaggedContainer {
  container_id: string
  node_id: string
  flags: SecurityFlag[]
}

export interface PrivilegedResponse {
  containers: FlaggedContainer[]
}

export type InterfaceClass = 'all' | 'loopback' | 'external'

export interface PortExposure {
  id: string
  node_id: string
  container_id: string
  host_ip: string
  host_port: number
  container_port: number
  protocol: string
  interface_class: InterfaceClass
  is_new: boolean
  notified_at?: string
  first_seen: string
  last_seen: string
}

export interface Listener {
  node_id: string
  protocol: string
  address: string
  port: number
  process: string
}

export interface PortsResponse {
  ports: PortExposure[]
  non_docker_listeners: Listener[]
}

export interface SecurityBadgesResponse {
  badges: Record<string, boolean>
}

export interface AcknowledgeRequest {
  container_id: string
  flag_type: string
  flag_key: string
  note?: string
}

// Activity Log types (Sub-project 9)

export interface ActivityEntry {
  id: string
  created_at: string
  user_id?: string
  username?: string
  action: string
  target_type?: string
  target_id?: string
  detail: unknown
  result: 'success' | 'error' | string
}

export interface ActivityListResponse {
  entries: ActivityEntry[]
  next_cursor: string
}

export interface ActivityActionInfo {
  action: string
  label: string
  category: string
  target: string
}

export interface ActivityActionsResponse {
  actions: ActivityActionInfo[]
}

export interface ActivityFilters {
  user?: string
  action?: string
  action_prefix?: string
  target_type?: string
  result?: string
  from?: string
  to?: string
  q?: string
}

// Volume Health types (Sub-project 7 / Feature 7)

export interface VolumeSamplePoint {
  sampled_at: string
  size_bytes: number
}

export type VolumeStatus = 'attached' | 'unused' | 'unknown'

export interface VolumeView {
  node_id: string
  name: string
  driver: string
  mountpoint: string
  created_at: string
  size_bytes: number
  ref_count: number
  status: VolumeStatus
  attached_containers: string[]
  over_threshold: boolean
  samples: VolumeSamplePoint[]
}

export interface VolumesResponse {
  volumes: VolumeView[]
}

// Network Topology types (Feature 29)

export interface TopologyEndpoint {
  container_id: string
  name: string
  ipv4_address: string
}

export interface TopologyNetwork {
  id: string
  name: string
  driver: string
  scope: string
  internal: boolean
  subnet: string
  gateway: string
  endpoints: TopologyEndpoint[]
}

export interface TopologyContainer {
  docker_id: string
  name: string
  status: string
  networks: string[]
  isolated: boolean
  host_network: boolean
}

export interface TopologyResponse {
  node_id: string
  networks: TopologyNetwork[]
  containers: TopologyContainer[]
}

// Dependency Graph types (Feature 16)

export type DepGraphNodeKind = 'container' | 'network' | 'volume'
export type DepGraphEdgeKind = 'network' | 'volume'

export interface DepGraphNode {
  id: string
  kind: DepGraphNodeKind
  label: string
  status?: string
  compose_project?: string
  driver?: string
}

export interface DepGraphEdge {
  source: string
  target: string
  kind: DepGraphEdgeKind
}

export interface DepGraph {
  node_id: string
  nodes: DepGraphNode[]
  edges: DepGraphEdge[]
}

// Resource Timeline types (Feature 9)

export interface MetricSample {
  sampled_at: string
  cpu_pct: number
  mem_bytes: number
  mem_limit_bytes: number
  disk_read_bytes: number
  disk_write_bytes: number
}

export interface MetricSpike {
  metric: 'cpu' | 'mem'
  from: string
  to: string
  peak: number
}

export interface MetricsResponse {
  samples: MetricSample[]
  spikes: MetricSpike[]
}

// Bulk Operations types (Feature 13)

export type BulkAction = 'start' | 'stop' | 'restart' | 'remove'

export type BulkResult = 'planned' | 'ok' | 'error' | 'skipped' | 'not_found'

export interface BulkResultItem {
  container_id: string
  name: string
  node_id: string
  status: string
  skip: boolean
  skip_reason?: string
  result: BulkResult
  error?: string
}

export interface BulkResponse {
  results: BulkResultItem[]
  dry_run: boolean
}

export interface BulkRequest {
  action: BulkAction
  container_ids: string[]
  dry_run: boolean
}

// Bookmarks types (Feature 24)

export type BookmarkResourceType = 'container' | 'node' | 'vm' | 'path' | 'file'

export interface Bookmark {
  id: string
  label: string
  resource_type: BookmarkResourceType
  resource_ref: string
  group_name: string
  order_index: number
}

export interface BookmarksResponse {
  bookmarks: Bookmark[]
}

export interface CreateBookmarkRequest {
  label: string
  resource_type: BookmarkResourceType
  resource_ref: string
  group_name?: string
}

// Notification Hooks types (Feature 26)

export type WebhookProvider = 'slack' | 'discord'

export interface Webhook {
  id: string
  name: string
  url: string
  provider: WebhookProvider
  triggers: string[]
  enabled: boolean
}

export interface WebhooksResponse {
  webhooks: Webhook[]
  available_triggers: string[]
}

export interface WebhookRequest {
  name: string
  url: string
  provider: WebhookProvider
  triggers: string[]
  enabled: boolean
}

// Template Library types (Feature 14)

export interface TemplateVar {
  name: string
  description: string
  default: string
}

export interface Template {
  id: string
  name: string
  description: string
  tags: string[]
  compose_yaml: string
  variables: TemplateVar[]
  version: number
}

export interface TemplateVersion {
  version: number
  compose_yaml: string
  variables: TemplateVar[]
  created_at: string
}

export interface TemplateWithVersions extends Template {
  versions: TemplateVersion[]
}

export interface TemplatesResponse {
  templates: Template[]
}

export interface TemplateCreateRequest {
  name: string
  description?: string
  tags?: string[]
  compose_yaml: string
  variables?: TemplateVar[]
}

export interface TemplateRenderRequest {
  variables: Record<string, string>
}

export interface TemplateRenderResponse {
  rendered: string
  unresolved: string[]
}

export interface TemplateDeployRequest {
  node_id: string
  dir?: string
  variables: Record<string, string>
}

export interface TemplateDeployResponse {
  path: string
  output: string
}

// Wake-on-LAN types (Feature 6)

export interface WOLConfig {
  mac: string
  broadcast: string
  port: number
}

export interface SetWOLRequest {
  mac: string
  broadcast?: string
  port?: number
}

// Update Assistant types (Feature 15)

export type UpdateStatus = 'up_to_date' | 'update_available' | 'unknown'

export interface ImageUpdate {
  container_id: string
  node_id: string
  image: string
  status: UpdateStatus
  current_digest: string
  latest_digest: string
  checked_at: string
}

export interface UpdatesResponse {
  updates: ImageUpdate[]
}

// Container Health Check types

export interface SetHealthcheckRequest {
  disable: boolean
  test?: string[]
  interval_sec?: number
  timeout_sec?: number
  start_period_sec?: number
  retries?: number
}

export interface SetHealthcheckResponse {
  new_container_id: string
}

export type HealthStatus = 'healthy' | 'unhealthy' | 'starting' | 'none'

export interface HealthLogEntry {
  start: string
  end: string
  exit_code: number
  output: string
}

export interface HealthReport {
  configured: boolean
  test: string[]
  interval_sec: number
  timeout_sec: number
  start_period_sec: number
  retries: number
  status: HealthStatus
  failing_streak: number
  log: HealthLogEntry[]
}

// Smart Search types (Feature 23)

export interface SearchNodeHit {
  id: string
  name: string
  type: string
}

export interface SearchContainerHit {
  id: string
  name: string
  node_id: string
  image: string
  status: string
}

export interface SearchVMHit {
  id: string
  name: string
  node_id: string
}

export interface SearchBookmarkHit {
  id: string
  label: string
  resource_type: string
  resource_ref: string
}

export interface SearchResponse {
  nodes: SearchNodeHit[]
  containers: SearchContainerHit[]
  vms: SearchVMHit[]
  bookmarks: SearchBookmarkHit[]
}

// Secrets Manager types (Feature 12)

export interface SecretKey {
  id: string
  key: string
}

export interface SecretGroup {
  id: string
  name: string
  description: string
  secrets: SecretKey[]
}

export interface SecretsResponse {
  groups: SecretGroup[]
}

export interface CreateSecretGroupRequest {
  name: string
  description?: string
}

export interface SetSecretRequest {
  key: string
  value: string
}

export interface ImportSecretsRequest {
  env: string
}

export interface ImportSecretsResponse {
  imported: number
}

export interface RevealResponse {
  key: string
  value: string
}

// SSH Key Audit types (Feature 21)

export interface SSHKey {
  user: string
  path: string
  type: string
  comment: string
  fingerprint: string
}

export interface SSHKeysResponse {
  keys: SSHKey[]
}

export interface DeleteSSHKeyRequest {
  path: string
  fingerprint: string
}

// Scheduled Tasks types (Feature 10)

export interface CronJob {
  user: string
  schedule: string
  command: string
  human: string
  raw: string
}

export interface SystemdTimer {
  unit: string
  activates: string
  next: string
  last: string
}

export interface ScheduleResponse {
  cron: CronJob[]
  timers: SystemdTimer[]
}

export interface SetCronRequest {
  user: string
  content: string
}

// CVE Scan types (Feature 20)

export interface ImageScan {
  image_digest: string
  image: string
  scanned_at: string
  critical: number
  high: number
  medium: number
  low: number
  unknown: number
}

export interface CVEScansResponse {
  available: boolean
  scans: ImageScan[]
}

export interface CVEVuln {
  cve_id: string
  severity: string
  package: string
  installed_version: string
  fixed_version: string
  title: string
}

export interface CVEDetailResponse {
  vulns: CVEVuln[]
}

// Script Runner types (Feature 27)

export interface Script {
  id: string
  name: string
  description: string
  content: string
}

export interface ScriptsResponse {
  scripts: Script[]
}

export interface ScriptCreateRequest {
  name: string
  description?: string
  content: string
}

export interface ScriptRunResult {
  node_id: string
  ok: boolean
  output: string
}

export interface RunScriptResponse {
  results: ScriptRunResult[]
}

export interface RunScriptRequest {
  node_ids: string[]
}

// Backup Orchestration types (Feature 28)

export type BackupStatus = 'running' | 'ok' | 'error'

export interface Backup {
  id: string
  node_id: string
  kind: string
  target: string
  dest_path: string
  size_bytes: number
  status: BackupStatus
  error?: string
  started_at: string
  finished_at?: string
}

export interface BackupsResponse {
  backups: Backup[]
}

export interface StartBackupRequest {
  volume: string
  dest_dir: string
}

export interface StartBackupResponse {
  backup_id: string
}

// Snapshot & Rollback types (Features 15 & 17)

export type SnapshotReason = 'manual' | 'pre-update' | 'pre-rollback'

export interface Snapshot {
  id: string
  reason: SnapshotReason
  image_ref: string
  image_digest?: string
  created_at: string
}

export interface SnapshotsResponse {
  snapshots: Snapshot[]
}

export interface SaveSnapshotResponse {
  id: string
  reason: SnapshotReason
  image_ref: string
  image_digest?: string
  created_at: string
}

export interface UpdateContainerResponse {
  new_container_id: string
}

export interface RollbackResponse {
  new_container_id: string
}

// AI Assistant types (Feature 31)

export type AIProvider = 'ollama' | 'claude' | 'claude-oauth' | 'openai' | 'gemini' | ''

export interface AIConfig {
  provider: AIProvider
  ollama_base_url: string
  ollama_model: string
  claude_model: string
  openai_model: string
  openai_base_url: string
  gemini_model: string
  has_api_key: boolean
  oauth_connected: boolean
  configured: boolean
}

export interface AIOAuthStartResponse {
  authorize_url: string
  verifier: string
  state: string
}

export interface SetAIConfigRequest {
  provider: AIProvider
  ollama_base_url?: string
  ollama_model?: string
  claude_model?: string
  openai_model?: string
  openai_base_url?: string
  gemini_model?: string
  api_key?: string
}

export type AITask = 'explain_log' | 'diagnose' | 'explain_config' | 'suggest_fix' | ''

export interface AIAskRequest {
  task?: AITask
  prompt: string
  context?: string
}

export interface AIAskResponse {
  answer: string
  provider: string
  input_tokens: number
  output_tokens: number
}

// Certificate Management types (Feature F4)

export interface CertInfo {
  id: string
  node_id: string
  source: string
  domain: string
  sans: string[]
  issuer: string
  path: string
  not_before?: string
  not_after?: string
  last_checked: string
}

export interface CertsResponse {
  certs: CertInfo[]
}

export interface RescanCertsResponse {
  certs: CertInfo[]
}

// Two-Factor Auth (TOTP) types (Feature 7 Phase 2)

export interface TwoFAStatus {
  enabled: boolean
}

export interface TwoFASetupResponse {
  secret: string
  provisioning_uri: string
  recovery_codes: string[]
}

export interface TwoFACodeRequest {
  code: string
}

// Agent Memory types (Feature F9)

export type MemoryScope = 'global' | 'node' | 'container'
export type MemorySource = 'user' | 'ai' | 'observed'

export interface Memory {
  id: string
  scope: MemoryScope
  scope_id?: string
  key: string
  value: string
  source: MemorySource
  confirmed: boolean
  created_at: string
  updated_at: string
}

export interface MemoryListResponse {
  memories: Memory[]
}

export interface CreateMemoryRequest {
  scope: MemoryScope
  scope_id?: string
  key: string
  value: string
}

export interface UpdateMemoryRequest {
  value?: string
  confirmed?: boolean
}

// Reverse Proxy Management types (Feature F1)

export interface ProxyCapabilities {
  list: boolean
  create: boolean
  update: boolean
  delete: boolean
}

export interface ProxyRule {
  id: string
  adapter_type: string
  source_host: string
  source_path?: string
  target_url: string
  ssl_enabled: boolean
  cert_id?: string
  auth_enabled: boolean
}

export interface SupportedProxy {
  name: string
  capabilities: ProxyCapabilities
}

export interface ProxyStatus {
  detected: string
  capabilities: ProxyCapabilities
  configured: boolean
  endpoint?: string
  has_token: boolean
  rules: ProxyRule[]
  rule_error?: string
  supported: SupportedProxy[]
}

export interface SetProxyConfigRequest {
  endpoint: string
  token?: string
}

// Feature Flags types

export interface FeatureFlag {
  key: string
  label: string
  enabled: boolean
  default: boolean
  description: string
}

export interface FeaturesResponse {
  features: FeatureFlag[]
}

export interface SetFeatureRequest {
  enabled: boolean
}

// Chat Integration types (Feature F8)

export interface ChatConfig {
  provider: 'telegram'
  has_token: boolean
  allowed_chats: number[]
}

export interface SetChatConfigRequest {
  allowed_chats: number[]
  token?: string
}

// AI Runbooks types (Feature F9)

export interface Runbook {
  id: string
  name: string
  description: string
  trigger_conditions: string[]
  steps: string[]
  requires_approval: boolean
  created_at: string
  updated_at: string
}

export interface RunbooksListResponse {
  runbooks: Runbook[]
}

export interface RunbookRequest {
  name: string
  description: string
  trigger_conditions: string[]
  steps: string[]
  requires_approval: boolean
}

// File Change Detection types (Feature 22)

export interface FileWatch {
  id: string
  node_id: string
  path: string
  recursive: boolean
  created_by: string
  created_at: string
}

export interface FileWatchesResponse {
  watches: FileWatch[]
}

export interface AddWatchRequest {
  path: string
  recursive: boolean
}

export interface ScanResponse {
  detected: number
}

export type FileEventType = 'create' | 'modify' | 'delete' | 'rename' | 'chmod' | string

export interface FileEvent {
  id: string
  node_id: string
  path: string
  event_type: FileEventType
  detected_at: string
}

export interface FileEventsResponse {
  events: FileEvent[]
}

// SSO Passthrough types (Feature F2)

export type SSOMethod = 'local' | 'totp' | 'oidc' | 'forward'

export interface SSOConfig {
  id: string
  node_id: string
  container_name: string
  enabled: boolean
  method: SSOMethod
  provider_url: string
  client_id: string
  allowed_groups: string[]
  session_duration_secs: number
  has_client_secret: boolean
  updated_at: string
}

export interface SSOListResponse {
  configs: SSOConfig[]
}

export interface SSOUpsertRequest {
  node_id: string
  container_name: string
  enabled: boolean
  method: SSOMethod
  provider_url?: string
  client_id?: string
  client_secret?: string
  allowed_groups: string[]
  session_duration_secs: number
}

// DNS Record Management types (Feature F3)

export type DnsRecordType = 'A' | 'AAAA' | 'CNAME' | 'PTR' | 'TXT' | 'SRV'

export interface DnsCapabilities {
  list: boolean
  create: boolean
  update: boolean
  delete: boolean
}

export interface DnsRecord {
  id: string
  adapter_type: string
  type: DnsRecordType
  name: string
  value: string
  ttl?: number
  comment?: string
}

export interface SupportedDns {
  name: string
  capabilities: DnsCapabilities
}

export interface DnsStatus {
  detected: string
  capabilities: DnsCapabilities
  configured: boolean
  endpoint?: string
  has_token: boolean
  records: DnsRecord[]
  record_error?: string
  supported: SupportedDns[]
}

export interface SetDnsConfigRequest {
  endpoint: string
  token?: string
}

