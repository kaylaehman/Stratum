package activity

// This file is the action/target taxonomy: the single source of truth for the
// audit action names emitters use and the filter UI enumerates. Adding a
// Phase-2 action (secret.access, template.deploy, container.start, ...) is a
// one-line addition to actionCatalog below.

// Action constants — every mutating endpoint emits one of these. Keep in sync
// with the emitters (api/*.go); the coverage-audit test asserts the wiring.
const (
	ActionAuthLogin  = "auth.login"
	ActionAuthLogout = "auth.logout"
	ActionSetupAdmin = "setup.admin"

	ActionNodeCreate = "node.create"
	ActionNodeUpdate = "node.update"
	ActionNodeDelete = "node.delete"
	ActionNodeProbe  = "node.probe"
	ActionNodeWake   = "node.wake"
	ActionNodeWOLSet = "node.wol_config"

	ActionFSWrite  = "fs.write"
	ActionFSUpload = "fs.upload"
	ActionFSMkdir  = "fs.mkdir"
	ActionFSRename = "fs.rename"
	ActionFSDelete = "fs.delete"

	ActionSecurityAcknowledge = "security.acknowledge"
	ActionSecurityAckRevoke   = "security.revoke_acknowledge"

	ActionVolumeRemove = "volume.remove"

	ActionContainerStart       = "container.start"
	ActionContainerStop        = "container.stop"
	ActionContainerRestart     = "container.restart"
	ActionContainerRemove      = "container.remove"
	ActionContainerUpdate      = "container.update"
	ActionContainerSnapshot    = "container.snapshot"
	ActionContainerRollback    = "container.rollback"
	ActionContainerHealthcheck = "container.healthcheck"

	ActionWebhookCreate = "webhook.create"
	ActionWebhookUpdate = "webhook.update"
	ActionWebhookDelete = "webhook.delete"

	ActionUpdatesRescan = "updates.rescan"

	ActionTemplateCreate = "template.create"
	ActionTemplateUpdate = "template.update"
	ActionTemplateDelete = "template.delete"
	ActionTemplateDeploy = "template.deploy"

	ActionSecretGroupCreate = "secret.group_create"
	ActionSecretGroupDelete = "secret.group_delete"
	ActionSecretSet         = "secret.set"
	ActionSecretDelete      = "secret.delete"
	ActionSecretReveal      = "secret.reveal"
	ActionSecretImport      = "secret.import"

	ActionSSHKeyDelete = "sshkey.delete"

	ActionCronSet = "cron.set"

	ActionCVEScan = "cve.scan"

	ActionScriptCreate = "script.create"
	ActionScriptUpdate = "script.update"
	ActionScriptDelete = "script.delete"
	ActionScriptRun    = "script.run"

	ActionBackupStart = "backup.start"

	ActionAIAsk    = "ai.ask"
	ActionAIConfig = "ai.config"

	ActionCertRescan = "cert.rescan"

	ActionMemoryCreate = "memory.create"
	ActionMemoryUpdate = "memory.update"
	ActionMemoryDelete = "memory.delete"

	ActionRunbookCreate = "runbook.create"
	ActionRunbookUpdate = "runbook.update"
	ActionRunbookDelete = "runbook.delete"

	ActionSkillCreate   = "skill.create"
	ActionSkillUpdate   = "skill.update"
	ActionSkillDelete   = "skill.delete"
	ActionSkillGenerate = "skill.generate"

	ActionProxyConfig   = "proxy.config"
	ActionDNSConfig     = "dns.config"
	ActionFeatureToggle = "feature.toggle"
	ActionChatConfig    = "chat.config"

	ActionWatchAdd    = "watch.add"
	ActionWatchDelete = "watch.delete"
	ActionWatchScan   = "watch.scan"

	ActionSSOConfig = "sso.config"
)

// Target type constants for ActivityEntry.TargetType.
const (
	TargetNode            = "node"
	TargetContainer       = "container"
	TargetFile            = "file"
	TargetSecret          = "secret"
	TargetAcknowledgement = "acknowledgement"
	TargetUser            = "user"
	TargetVolume          = "volume"
	TargetWebhook         = "webhook"
	TargetTemplate        = "template"
	TargetSSHKey          = "sshkey"
	TargetScript          = "script"
	TargetAI              = "ai"
	TargetSkill           = "skill"
)

// ActionInfo describes one action for the filter UI: a stable name, a
// human-readable label, the category (action name prefix, e.g. "fs") used for
// prefix filtering, and the target type it most commonly acts on.
type ActionInfo struct {
	Action   string `json:"action"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Target   string `json:"target"`
}

// actionCatalog is the ordered taxonomy. Order is the display order in the UI.
var actionCatalog = []ActionInfo{
	{ActionAuthLogin, "User logged in", "auth", TargetUser},
	{ActionAuthLogout, "User logged out", "auth", TargetUser},
	{ActionSetupAdmin, "Initial admin created", "setup", TargetUser},

	{ActionNodeCreate, "Host added", "node", TargetNode},
	{ActionNodeUpdate, "Host updated", "node", TargetNode},
	{ActionNodeDelete, "Host removed", "node", TargetNode},
	{ActionNodeProbe, "Host re-probed", "node", TargetNode},
	{ActionNodeWake, "Wake-on-LAN sent", "node", TargetNode},
	{ActionNodeWOLSet, "Wake-on-LAN configured", "node", TargetNode},

	{ActionFSWrite, "File written", "fs", TargetFile},
	{ActionFSUpload, "File uploaded", "fs", TargetFile},
	{ActionFSMkdir, "Directory created", "fs", TargetFile},
	{ActionFSRename, "File renamed", "fs", TargetFile},
	{ActionFSDelete, "File deleted", "fs", TargetFile},

	{ActionSecurityAcknowledge, "Security flag acknowledged", "security", TargetContainer},
	{ActionSecurityAckRevoke, "Security acknowledgement revoked", "security", TargetAcknowledgement},

	{ActionVolumeRemove, "Volume removed", "volume", TargetVolume},

	{ActionContainerStart, "Container started", "container", TargetContainer},
	{ActionContainerStop, "Container stopped", "container", TargetContainer},
	{ActionContainerRestart, "Container restarted", "container", TargetContainer},
	{ActionContainerRemove, "Container removed", "container", TargetContainer},
	{ActionContainerUpdate, "Container updated (image)", "container", TargetContainer},
	{ActionContainerSnapshot, "Snapshot saved", "container", TargetContainer},
	{ActionContainerRollback, "Container rolled back", "container", TargetContainer},
	{ActionContainerHealthcheck, "Healthcheck edited", "container", TargetContainer},

	{ActionWebhookCreate, "Notification webhook created", "webhook", TargetWebhook},
	{ActionWebhookUpdate, "Notification webhook updated", "webhook", TargetWebhook},
	{ActionWebhookDelete, "Notification webhook deleted", "webhook", TargetWebhook},

	{ActionUpdatesRescan, "Image updates re-checked", "updates", TargetContainer},

	{ActionTemplateCreate, "Template created", "template", TargetTemplate},
	{ActionTemplateUpdate, "Template updated", "template", TargetTemplate},
	{ActionTemplateDelete, "Template deleted", "template", TargetTemplate},
	{ActionTemplateDeploy, "Template deployed", "template", TargetTemplate},

	{ActionSecretGroupCreate, "Secret group created", "secret", TargetSecret},
	{ActionSecretGroupDelete, "Secret group deleted", "secret", TargetSecret},
	{ActionSecretSet, "Secret set", "secret", TargetSecret},
	{ActionSecretDelete, "Secret deleted", "secret", TargetSecret},
	{ActionSecretReveal, "Secret revealed", "secret", TargetSecret},
	{ActionSecretImport, "Secrets imported from .env", "secret", TargetSecret},

	{ActionSSHKeyDelete, "SSH key removed", "sshkey", TargetSSHKey},

	{ActionCronSet, "Crontab updated", "cron", TargetNode},

	{ActionCVEScan, "Image CVE scan", "cve", TargetContainer},

	{ActionScriptCreate, "Script created", "script", TargetScript},
	{ActionScriptUpdate, "Script updated", "script", TargetScript},
	{ActionScriptDelete, "Script deleted", "script", TargetScript},
	{ActionScriptRun, "Script run", "script", TargetScript},

	{ActionBackupStart, "Backup started", "backup", TargetNode},

	{ActionAIAsk, "AI assistant query", "ai", TargetAI},
	{ActionAIConfig, "AI provider configured", "ai", TargetAI},

	{ActionCertRescan, "Certificates re-scanned", "cert", TargetNode},

	{ActionMemoryCreate, "AI memory added", "memory", TargetAI},
	{ActionMemoryUpdate, "AI memory updated", "memory", TargetAI},
	{ActionMemoryDelete, "AI memory deleted", "memory", TargetAI},
	{ActionRunbookCreate, "Runbook created", "runbook", TargetAI},
	{ActionRunbookUpdate, "Runbook updated", "runbook", TargetAI},
	{ActionRunbookDelete, "Runbook deleted", "runbook", TargetAI},

	{ActionSkillCreate, "Troubleshooting skill created", "skill", TargetSkill},
	{ActionSkillUpdate, "Troubleshooting skill updated", "skill", TargetSkill},
	{ActionSkillDelete, "Troubleshooting skill deleted", "skill", TargetSkill},
	{ActionSkillGenerate, "Troubleshooting skill drafted by AI", "skill", TargetSkill},

	{ActionProxyConfig, "Reverse-proxy endpoint configured", "proxy", TargetNode},
	{ActionDNSConfig, "DNS endpoint configured", "dns", TargetNode},
	{ActionFeatureToggle, "Feature toggled", "feature", TargetUser},
	{ActionChatConfig, "Chat bot configured", "chat", TargetUser},
	{ActionWatchAdd, "File watch added", "watch", TargetNode},
	{ActionWatchDelete, "File watch removed", "watch", TargetNode},
	{ActionWatchScan, "File change scan", "watch", TargetNode},
	{ActionSSOConfig, "SSO access configured", "sso", TargetContainer},
}

var actionByName = func() map[string]ActionInfo {
	m := make(map[string]ActionInfo, len(actionCatalog))
	for _, a := range actionCatalog {
		m[a.Action] = a
	}
	return m
}()

// Catalog returns the taxonomy in display order (for GET /api/activity/actions).
func Catalog() []ActionInfo { return actionCatalog }

// LookupAction returns the ActionInfo for a name and whether it is known.
func LookupAction(name string) (ActionInfo, bool) {
	a, ok := actionByName[name]
	return a, ok
}
