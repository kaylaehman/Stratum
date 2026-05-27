# Stratum Skill Library — Index

Auto-generated index of every container runbook in this library. The agent
loader uses the YAML files directly; this index is for human browsing and
PR review.

See `SCHEMA.md` for the file format and `assets/skills/<category>/<id>.yaml`
for each entry.

## Summary by category

| Category | Skills |
|---|---|
| `ai` | 7 |
| `analytics` | 4 |
| `automation` | 5 |
| `backup` | 3 |
| `communication` | 7 |
| `dashboard` | 4 |
| `data` | 3 |
| `database` | 10 |
| `development` | 10 |
| `documents` | 1 |
| `ebooks` | 4 |
| `email` | 4 |
| `files` | 8 |
| `finance` | 5 |
| `games` | 5 |
| `identity` | 2 |
| `media` | 21 |
| `monitoring` | 13 |
| `network` | 18 |
| `passwords` | 1 |
| `photos` | 2 |
| `productivity` | 20 |
| `rss` | 3 |
| `security` | 11 |
| `smarthome` | 4 |
| `time` | 2 |
| `torrent` | 3 |
| `voip` | 2 |
| `weather` | 2 |
| `web` | 2 |
| **Total** | **186** |

## Skills

| Skill | Category | Primary image | Issues | LSIO | Description |
|---|---|---|---|---|---|
| `flowise` | ai | `flowiseai/flowise` | 4 |  | Drag-and-drop visual builder for LLM workflows and AI applications |
| `langflow` | ai | `langflowai/langflow` | 5 |  | Visual editor for building and testing LangChain workflows and applications |
| `langfuse` | ai | `langfuse/langfuse` | 4 |  | Open-source observability platform for monitoring and debugging LLM applications |
| `litellm` | ai | `ghcr.io/berriai/litellm` | 5 |  | LLM gateway that proxies requests to multiple providers (OpenAI, Anthropic, Azure, local models) |
| `ollama` | ai | `ollama/ollama` | 4 |  | Run large language models locally without internet dependency; supports GPU acceleration |
| `openclaw` | ai | `ghcr.io/openclaw/openclaw` | 3 |  | Multi-channel AI control plane for orchestrating Claude API calls and routing across different AI backends |
| `openwebui` | ai | `ghcr.io/open-webui/open-webui` | 4 |  | Web-based chat interface for Ollama and OpenAI-compatible APIs with user authentication |
| `matomo` | analytics | `matomo` | 4 |  | Open-source Google Analytics alternative — full data ownership and control |
| `plausible` | analytics | `ghcr.io/plausible/analytics` | 3 |  | Privacy-focused web analytics platform — GDPR-compliant alternative to Google Analytics |
| `posthog` | analytics | `posthog/posthog` | 4 |  | Open-source product analytics — event capture, feature flags, session replay, multi-service stack |
| `umami` | analytics | `ghcr.io/umami-software/umami` | 4 |  | Lightweight web analytics platform — no cookies, simple single-container deployment |
| `diun` | automation | `crazymax/diun` | 2 |  | Docker image update notifier that watches registries and sends notifications via webhook |
| `homeassistant` | automation | `ghcr.io/home-assistant/home-assistant` | 4 |  | Open-source home automation platform for controlling smart devices and automations |
| `n8n` | automation | `n8nio/n8n` | 4 |  | Open-source workflow automation platform for connecting apps and services |
| `nodered` | automation | `nodered/node-red` | 2 |  | Visual programming tool for wiring together hardware, APIs, and services |
| `watchtower` | automation | `containrrr/watchtower` | 2 |  | Auto-updates running Docker containers when their images change in the registry |
| `duplicati` | backup | `lscr.io/linuxserver/duplicati` | 4 | ✓ | Encrypted, incremental backup tool with web UI and cloud storage support |
| `kopia` | backup | `kopia/kopia` | 4 |  | Fast, deduplicating backup tool with client-side encryption and web UI |
| `restic` | backup | `restic/restic` | 4 |  | Lightweight CLI backup tool with encryption, deduplication, and scheduling support |
| `gotify` | communication | `gotify/server` | 4 |  | Self-hosted push notification server with REST API and mobile apps |
| `jitsi` | communication | `jitsi/web` | 4 |  | Self-hosted video conferencing platform with WebRTC, secure rooms, and multi-participant support |
| `listmonk` | communication | `listmonk/listmonk` | 5 |  | Self-hosted email newsletter platform with list management and campaign delivery |
| `matrix-synapse` | communication | `matrixdotorg/synapse` | 4 |  | Reference home server for the Matrix decentralized communication protocol |
| `mattermost` | communication | `mattermost/mattermost-team-edition` | 4 |  | Self-hosted team chat platform with messaging, file sharing, and integrations |
| `ntfy` | communication | `binwiederhier/ntfy` | 4 |  | Simple, self-hosted push notification server with minimal dependencies |
| `rocketchat` | communication | `rocket.chat` | 4 |  | Open-source team chat with messaging, video calls, and enterprise features |
| `heimdall` | dashboard | `linuxserver/heimdall` | 4 | ✓ | An elegant application dashboard and launcher for your services |
| `homepage` | dashboard | `ghcr.io/gethomepage/homepage` | 3 |  | Highly customizable homepage and application dashboard with Docker and service integrations |
| `homer` | dashboard | `b4bz/homer` | 4 |  | A very simple static homepage for your homelab server with bookmark management |
| `organizr` | dashboard | `organizr/organizr` | 4 |  | Tab-based dashboard and organizer for managing and accessing multiple services |
| `baserow` | data | `baserow/baserow` | 5 |  | Open-source Airtable alternative for building collaborative databases and spreadsheets |
| `nocodb` | data | `nocodb/nocodb` | 4 |  | Open-source Airtable alternative for creating and managing databases with spreadsheet-like UI |
| `teable` | data | `ghcr.io/teableio/teable` | 3 |  | No-code database platform with spreadsheet-like UI, requires PostgreSQL backend |
| `adminer` | database | `adminer` | 3 |  | Lightweight single-file database administration tool supporting multiple database systems |
| `couchdb` | database | `couchdb` | 3 |  | Document-oriented NoSQL database with HTTP API, peer-to-peer replication, and eventual consistency |
| `mariadb` | database | `mariadb` | 4 | ✓ | Open-source relational database, MySQL-compatible fork, widely used in self-hosted stacks |
| `mongodb` | database | `mongo` | 3 |  | Document-oriented NoSQL database with rich querying and aggregation, widely used in JavaScript-heavy stacks |
| `mysql` | database | `mysql` | 3 |  | Open-source relational database, predecessor to MariaDB, widely used in legacy and WordPress stacks |
| `pgadmin` | database | `dpage/pgadmin4` | 4 |  | Web-based PostgreSQL administration and management interface |
| `phpmyadmin` | database | `phpmyadmin` | 5 |  | Web-based MySQL and MariaDB administration interface |
| `postgresql` | database | `postgres` | 4 |  | Advanced open-source relational database with JSONB, full-text search, and superior query optimizer |
| `redis` | database | `redis` | 4 |  | High-performance in-memory data store for caching, sessions, queues, and real-time workloads |
| `valkey` | database | `valkey/valkey` | 4 |  | Redis fork by Linux Foundation; drop-in Redis replacement with active community maintenance and feature development |
| `code-server` | development | `codercom/code-server` | 4 |  | VS Code running in a browser, accessible remotely for web-based development |
| `dokploy` | development | `dokploy/dokploy` | 3 |  | Self-hosted PaaS platform for managing Docker hosts, applications, and deployments |
| `forgejo` | development | `forgejo/forgejo` | 4 |  | Community-driven Git service fork from Gitea with web UI and SSH access |
| `gitea` | development | `gitea/gitea` | 4 |  | Lightweight self-hosted Git service with web UI and SSH access |
| `jenkins` | development | `jenkins/jenkins` | 4 |  | Open-source automation server for building, testing, and deploying applications |
| `outline` | development | `outlinewiki/outline` | 4 |  | Team knowledge base and wiki platform with real-time collaboration |
| `registry` | development | `registry` | 3 |  | Self-hosted Docker image registry for private image storage and distribution |
| `sonarqube` | development | `sonarqube` | 4 |  | Code quality and security analysis platform for source code inspection |
| `verdaccio` | development | `verdaccio/verdaccio` | 3 |  | Lightweight npm private registry and proxy for caching public packages |
| `woodpecker-ci` | development | `woodpeckerci/woodpecker-server` | 4 |  | Lightweight CI/CD server for automated testing and deployment pipelines |
| `paperless-ngx` | documents | `ghcr.io/paperless-ngx/paperless-ngx` | 4 |  | Open-source document management system that transforms scanned documents and PDFs into searchable archives |
| `calibre` | ebooks | `linuxserver/calibre` | 4 | ✓ | Desktop ebook manager with web UI and content server for organizing and managing digital books |
| `calibre-web` | ebooks | `linuxserver/calibre-web` | 4 | ✓ | Web-based ebook library reader and manager, lighter alternative to full Calibre desktop |
| `kavita` | ebooks | `kizaing/kavita` | 5 |  | Full-featured server for reading comics, manga, ebooks, and light novels with advanced features |
| `komga` | ebooks | `gotson/komga` | 4 |  | Lightweight server for reading and managing comics and manga collections |
| `mailcow` | email | `mailcow/mailcow-dockerized` | 4 |  | Full-featured open-source mail server suite with web UI, spam/virus scanning, and RBAC |
| `mailu` | email | `ghcr.io/mailu/admin` | 5 |  | Simple yet full-featured mail server with web administration and webmail UI |
| `roundcube` | email | `roundcube/roundcubemail` | 4 |  | IMAP webmail client with multi-user support and plugin ecosystem |
| `stalwart` | email | `stalwartlabs/mail-server` | 4 |  | All-in-one SMTP, IMAP, and JMAP mail server written in Rust |
| `filebrowser` | files | `filebrowser/filebrowser` | 2 |  | Lightweight web file manager with multi-user support |
| `minio` | files | `minio/minio` | 3 |  | High-performance S3-compatible object storage |
| `nextcloud` | files | `nextcloud` | 4 |  | Self-hosted productivity suite — file sync, calendar, contacts, collaborative editing |
| `owncloud` | files | `owncloud/server` | 2 |  | Self-hosted file sync and share platform (predecessor to Nextcloud) |
| `rustfs` | files | `rustfs/rustfs` | 2 |  | High-performance S3-compatible object storage written in Rust |
| `seafile` | files | `seafileltd/seafile-mc` | 2 |  | Open-source file sync and share with end-to-end encryption support |
| `sftpgo` | files | `drakkan/sftpgo` | 2 |  | SFTP/FTP/WebDAV/HTTP server with web admin UI and per-user virtual filesystem |
| `syncthing` | files | `linuxserver/syncthing` | 2 | ✓ | Decentralized peer-to-peer file synchronization |
| `actual-budget` | finance | `actualbudget/actual-server` | 4 |  | Community-maintained zero-based budgeting tool with local data storage and sync |
| `akaunting` | finance | `akaunting/akaunting` | 4 |  | Open-source accounting software with invoicing, expense tracking, and financial reporting |
| `fireflyiii` | finance | `fireflyiii/core` | 4 |  | Open-source personal finance manager for budgeting, expense tracking, and financial planning |
| `gnucash` | finance | `gnucash/gnucash` | 2 |  | Double-entry accounting application, containerized for headless use |
| `homebank` | finance | `linuxserver/homebank` | 2 | ✓ | Personal accounting application (LinuxServer KasmVNC desktop variant) |
| `minecraft` | games | `itzg/minecraft-server` | 4 |  | Self-hosted Minecraft Java Edition multiplayer server with configurable difficulty, gamemode, and memory limits |
| `pterodactyl-panel` | games | `ghcr.io/pterodactyl/panel` | 4 |  | Web-based control panel for managing game servers, users, and nodes across Pterodactyl infrastructure |
| `pterodactyl-wings` | games | `ghcr.io/pterodactyl/wings` | 4 |  | Daemon that runs on each node to execute and manage individual game server instances controlled by Pterodactyl Panel |
| `romm` | games | `rommapp/romm` | 4 |  | Web interface for organizing, managing, and playing ROM collections with metadata scraping and multi-platform support |
| `valheim` | games | `lloesche/valheim-server` | 3 |  | Self-hosted Valheim multiplayer server with world persistence, configurable difficulty, and player limits |
| `keycloak` | identity | `quay.io/keycloak/keycloak` | 4 |  | Open-source identity and access management platform supporting OIDC, SAML, and Kerberos |
| `zitadel` | identity | `ghcr.io/zitadel/zitadel` | 4 |  | Cloud-native identity provider with OpenID Connect, SAML, and JWT support |
| `bazarr` | media | `linuxserver/bazarr` | 3 | ✓ | Companion to Sonarr and Radarr that downloads and manages subtitles for media |
| `declutarr` | media | `ghcr.io/manimatter/declutarr` | 2 |  | Queue cleaner for Sonarr, Radarr, Lidarr, and Readarr — removes stalled, slow, and failed downloads |
| `emby` | media | `emby/embyserver` | 4 |  | Proprietary media server for organizing and streaming video, music, and photos |
| `flaresolverr` | media | `ghcr.io/flaresolverr/flaresolverr` | 2 |  | Proxy server that solves Cloudflare and DDoS-Guard challenges for indexer requests |
| `jackett` | media | `linuxserver/jackett` | 2 | ✓ | API support proxy for torrent trackers — predecessor to Prowlarr, still used by some setups |
| `jellyfin` | media | `jellyfin/jellyfin` | 4 | ✓ | Open-source media server for organizing and streaming video, music, and photos |
| `jellyseerr` | media | `fallenbagel/jellyseerr` | 2 |  | Jellyfin and Emby request management — fork of Overseerr |
| `lidarr` | media | `linuxserver/lidarr` | 2 | ✓ | Music collection manager (Sonarr/Radarr equivalent for music) |
| `metube` | media | `ghcr.io/alexta69/metube` | 2 |  | Web UI for yt-dlp — download videos from YouTube and other sites |
| `overseerr` | media | `sctx/overseerr` | 2 |  | Media request management and discovery for Plex with Sonarr/Radarr integration |
| `plex` | media | `plexinc/pms-docker` | 4 | ✓ | Media server for managing and streaming video, music, and photos |
| `prowlarr` | media | `linuxserver/prowlarr` | 3 | ✓ | Indexer aggregator and proxy that syncs torrent and usenet indexers to Sonarr, Radarr, Lidarr, and Readarr |
| `qbittorrent` | media | `linuxserver/qbittorrent` | 4 | ✓ | Free, open-source BitTorrent client with web UI and Sonarr/Radarr integration |
| `radarr` | media | `linuxserver/radarr` | 4 | ✓ | Movie collection manager that monitors releases and automates download and import |
| `readarr` | media | `linuxserver/readarr` | 2 | ✓ | Ebook and audiobook collection manager (Sonarr/Radarr equivalent for books) |
| `recyclarr` | media | `ghcr.io/recyclarr/recyclarr` | 2 |  | Syncs TRaSH Guides quality definitions and custom formats into Sonarr and Radarr |
| `sonarr` | media | `linuxserver/sonarr` | 4 | ✓ | TV series collection manager that monitors RSS feeds and automates download and import |
| `tautulli` | media | `linuxserver/tautulli` | 2 | ✓ | Monitoring and statistics dashboard for Plex Media Server |
| `tdarr` | media | `haveagitgat/tdarr` | 2 |  | Distributed media transcoding system with web UI, plugin-driven workflows, and remote workers |
| `transmission` | media | `linuxserver/transmission` | 3 | ✓ | Lightweight BitTorrent client with web UI |
| `unmanic` | media | `josh5/unmanic` | 2 |  | Automated media library optimizer — transcodes and normalizes files in the background |
| `alertmanager` | monitoring | `prom/alertmanager` | 4 |  | Alert management and routing system that receives alerts from Prometheus and sends notifications to configured receivers |
| `beszel` | monitoring | `henrygd/beszel` | 2 |  | Lightweight centralized server monitoring hub for aggregating metrics from Beszel agents |
| `beszel-agent` | monitoring | `henrygd/beszel-agent` | 3 |  | Agent for Beszel monitoring hub; collects system metrics and reports to central hub |
| `dozzle` | monitoring | `amir20/dozzle` | 2 |  | Lightweight web-based real-time Docker container log viewer |
| `glances` | monitoring | `nicolargo/glances` | 2 |  | Cross-platform system monitoring with web UI and REST API |
| `grafana` | monitoring | `grafana/grafana` | 4 |  | Open-source visualization and alerting platform for time-series metrics and operational dashboards |
| `healthchecks` | monitoring | `linuxserver/healthchecks` | 4 | ✓ | Self-hosted status page and monitoring service (healthchecks.io alternative) |
| `influxdb` | monitoring | `influxdb` | 4 |  | High-performance time-series database for metrics and telemetry ingestion |
| `loki` | monitoring | `grafana/loki` | 4 |  | Scalable log aggregation system with label-based indexing and efficient storage |
| `netdata` | monitoring | `netdata/netdata` | 8 |  | Real-time performance monitoring with system observability for containers and hosts |
| `prometheus` | monitoring | `prom/prometheus` | 4 |  | Open-source monitoring and alerting system for collecting and storing time-series metrics |
| `scrutiny` | monitoring | `ghcr.io/analogj/scrutiny` | 3 |  | S.M.A.R.T. disk health monitoring and analysis tool for proactive drive failure detection |
| `uptime-kuma` | monitoring | `louislam/uptime-kuma` | 2 |  | Self-hosted uptime monitoring with status pages and notifications |
| `adguardhome` | network | `adguard/adguardhome` | 4 |  | Network-wide DNS ad blocker and parental control solution with web interface |
| `caddy` | network | `caddy` | 3 |  | Automatic HTTPS reverse proxy with built-in Let's Encrypt support and dynamic config reloading |
| `cloudflared` | network | `cloudflare/cloudflared` | 4 |  | Cloudflare Tunnel client for secure ingress without port forwarding |
| `dockerproxy` | network | `ghcr.io/tecnativa/docker-socket-proxy` | 4 |  | Security layer restricting Docker socket access via whitelist of allowed endpoints |
| `dockge` | network | `louislam/dockge` | 3 |  | Lightweight Docker Compose stack manager and editor with UI |
| `gluetun` | network | `qmcgaw/gluetun` | 4 |  | Lightweight VPN client container for routing traffic through OpenVPN or WireGuard |
| `headscale` | network | `headscale/headscale` | 5 |  | Self-hosted Tailscale control server; coordinates WireGuard peer discovery and routing for on-premises or private mesh networks |
| `netbird` | network | `netbirdio/management` | 5 |  | Open-source, self-hosted WireGuard mesh VPN with management dashboard, signal server, and TURN relay |
| `netmaker` | network | `gravitl/netmaker` | 5 |  | Self-hosted mesh network controller with UI dashboard for WireGuard node registration and ACL-based policy |
| `nginx` | network | `nginx` | 2 |  | High-performance HTTP server and reverse proxy |
| `nginx-proxy-manager` | network | `jc21/nginx-proxy-manager` | 4 |  | User-friendly reverse proxy and SSL certificate manager with web-based UI |
| `pihole` | network | `pihole/pihole` | 4 |  | Lightweight DNS sinkhole that blocks ads and trackers across your entire network |
| `portainer` | network | `portainer/portainer-ce` | 4 |  | Docker/Kubernetes container management UI with edge agent support |
| `speedtest-tracker` | network | `linuxserver/speedtest-tracker` | 4 | ✓ | Lightweight speedtest monitoring tool with persistent result tracking and UI dashboard |
| `tailscale` | network | `tailscale/tailscale` | 5 |  | WireGuard-based mesh VPN for secure node-to-node connectivity with Tailscale infrastructure |
| `traefik` | network | `traefik` | 4 |  | Modern reverse proxy and load balancer with automatic Let's Encrypt SSL, Docker integration, and dashboard |
| `unbound` | network | `mvance/unbound` | 3 |  | Lightweight recursive DNS resolver commonly deployed as upstream for Pi-hole or AdGuard Home |
| `wg-easy` | network | `ghcr.io/wg-easy/wg-easy` | 2 |  | Self-hosted WireGuard VPN with easy web UI for peer management |
| `passbolt` | passwords | `passbolt/passbolt` | 4 |  | Open-source team password manager with role-based access control |
| `immich` | photos | `ghcr.io/immich-app/immich-server` | 4 |  | Self-hosted photo and video management application with AI-powered search, mobile sync, and machine learning features |
| `photoprism` | photos | `photoprism/photoprism` | 4 |  | AI-powered photo management and browsing system with support for storage management, backup, and mobile sync |
| `appflowy` | productivity | `appflowyio/appflowy_cloud` | 4 |  | Self-hosted Notion-like collaborative workspace backend |
| `bookstack` | productivity | `linuxserver/bookstack` | 4 | ✓ | Self-hosted wiki and documentation management platform with role-based access |
| `docmost` | productivity | `docmost/docmost` | 3 |  | Self-hosted collaborative document editor and wiki platform |
| `documenso` | productivity | `documenso/documenso` | 3 |  | Open-source document signing and agreement platform alternative to DocuSign |
| `excalidraw` | productivity | `excalidraw/excalidraw` | 3 |  | Virtual whiteboard application for collaborative drawing and diagramming |
| `grocy` | productivity | `linuxserver/grocy` | 4 | ✓ | Self-hosted groceries and household management system |
| `hedgedoc` | productivity | `linuxserver/hedgedoc` | 4 | ✓ | Collaborative markdown editor and note-taking platform |
| `hoarder` | productivity | `ghcr.io/hoarder-app/hoarder` | 4 |  | Self-hosted bookmark and archive manager with AI-powered tagging and full-text search |
| `it-tools` | productivity | `corentinth/it-tools` | 2 |  | Static web application with utilities for IT professionals—base64, hash, regex, UUID, JSON, and more |
| `joplin` | productivity | `florider89/joplin-server` | 4 |  | End-to-end encrypted note-taking and task management with sync server |
| `kanboard` | productivity | `kanboard/kanboard` | 4 |  | Lightweight open-source Kanban board for project and task management |
| `linkwarden` | productivity | `linuxserver/linkwarden` | 4 | ✓ | Self-hosted bookmark manager with full-page web archive snapshots |
| `mealie` | productivity | `ghcr.io/mealie-recipes/mealie` | 4 |  | Self-hosted recipe manager and meal planner with grocery list generation |
| `memos` | productivity | `neosmemo/memos` | 3 |  | Lightweight, fast note-taking and micro-blogging platform |
| `monica` | productivity | `monicahq/monicahq` | 3 |  | Personal CRM for managing relationships, contacts, and life events |
| `notifuse` | productivity | `notifuse/notifuse` | 2 |  | Self-hosted multi-channel notification platform (email, push, SMS) |
| `plane` | productivity | `makeplane/plane` | 4 |  | Open-source Jira/Linear alternative for team project management |
| `stirling-pdf` | productivity | `frooodle/s-pdf` | 2 |  | Web-based PDF manipulation tool suite with OCR, compression, and document utilities |
| `vikunja` | productivity | `vikunja/vikunja` | 4 |  | Self-hosted open-source task and project management platform |
| `wekan` | productivity | `wekan/wekan` | 4 |  | Open-source Trello-like Kanban board with team collaboration and card tracking |
| `freshrss` | rss | `freshrss/freshrss` | 3 |  | Self-hosted RSS reader with multi-user support and web-based interface |
| `miniflux` | rss | `miniflux/miniflux` | 4 |  | Minimalist and opinionated RSS reader with PostgreSQL backend |
| `ttyrss` | rss | `linuxserver/tt-rss` | 4 | ✓ | Feature-rich RSS reader with plugin system and comprehensive feed management |
| `authelia` | security | `authelia/authelia` | 5 |  | Single sign-on and 2FA authentication server — lightweight SSO/OIDC provider for reverse proxies |
| `authentik` | security | `ghcr.io/goauthentik/server` | 5 |  | Enterprise-grade authentication, SSO, and SAML/OIDC provider for self-hosted environments |
| `bitwarden` | security | `bitwarden/self-host` | 5 |  | Official Bitwarden self-hosted password manager with identity, API, and organizational support |
| `crowdsec` | security | `crowdsecurity/crowdsec` | 4 |  | Collaborative threat intelligence and DDoS detection — real-time IP reputation and behavior analysis |
| `fail2ban` | security | `linuxserver/fail2ban` | 3 | ✓ | Brute-force attack prevention by monitoring logs and banning IPs |
| `lldap` | security | `lldap/lldap` | 4 |  | Lightweight LDAP server for centralized user and group authentication in homelab environments |
| `openvpn` | security | `kylemanna/openvpn` | 4 |  | Open-source VPN server for secure remote access |
| `searxng` | security | `searxng/searxng` | 3 |  | Privacy-respecting meta-search engine — aggregates results from multiple search engines without tracking |
| `vaultwarden` | security | `vaultwarden/server` | 4 |  | Self-hosted Bitwarden-compatible password manager and vault |
| `wazuh` | security | `wazuh/wazuh-manager` | 3 |  | Open-source SIEM platform for centralized endpoint monitoring, log aggregation, and threat detection |
| `wireguard` | security | `linuxserver/wireguard` | 2 | ✓ | Modern, fast VPN server using WireGuard protocol |
| `esphome` | smarthome | `esphome/esphome` | 5 |  | Build and manage firmware for ESP8266 and ESP32 devices with YAML configuration |
| `frigate` | smarthome | `ghcr.io/blakeblackshear/frigate` | 6 |  | Real-time object detection for IP cameras with GPU acceleration support |
| `mosquitto` | smarthome | `eclipse-mosquitto` | 5 |  | Lightweight MQTT message broker for IoT and home automation |
| `zigbee2mqtt` | smarthome | `koenkk/zigbee2mqtt` | 4 |  | Bridge between Zigbee devices and MQTT, enabling home automation integration |
| `kimai` | time | `kimai/kimai2` | 3 |  | Web-based time tracking and project management for teams and freelancers |
| `timetagger` | time | `ghcr.io/timetagger/timetagger` | 3 |  | Web-based time tracking tool with flexible logging and reporting |
| `deluge` | torrent | `linuxserver/deluge` | 4 | ✓ | Full-featured BitTorrent client with web UI and plugin architecture |
| `nzbget` | torrent | `linuxserver/nzbget` | 4 | ✓ | Lightweight usenet downloader and post-processor, alternative to SABnzbd |
| `sabnzbd` | torrent | `linuxserver/sabnzbd` | 4 | ✓ | Usenet downloader and post-processor with automatic unpacking and repair |
| `asterisk` | voip | `andrius/asterisk` | 3 |  | Open-source SIP PBX telephony server for voice communications |
| `freepbx` | voip | `tiredofit/freepbx` | 3 |  | Web-based management GUI for Asterisk PBX systems |
| `meteobridge` | weather | `weatherflow/weatherflow-mqtt` | 4 |  | Docker-based weather station data integration and MQTT publishing for multi-source weather networks |
| `weewx` | weather | `mitct02/weewx` | 4 |  | Personal weather station software for collecting, processing, and displaying weather data |
| `ghost` | web | `ghost` | 4 |  | Lightweight, open-source publishing platform for blogs and newsletters |
| `wordpress` | web | `wordpress` | 4 |  | Open-source content management system for blogs and websites |
