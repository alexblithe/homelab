# Storage & Disaster Recovery Architecture

> Living document — captures the storage strategy, backup pipeline, and DR plan for the homelab k3s cluster.

## Design Principles

1. **Keep it simple.** No distributed storage (Longhorn, Ceph, Rook). The complexity isn't justified for a homelab with 2 local nodes on single NVMe drives.
2. **Backup, don't replicate at the block level.** Use application-level replication (CNPG) where it matters, and snapshot-based backups (Volsync/restic) for everything else.
3. **Dataloss is unacceptable.** Every piece of meaningful state must be recoverable — either from git (config-as-code) or from S3 backups.
4. **Egress-free by design.** Data flows *into* AWS (free ingress). Egress only happens during DR restore, which is a one-time acceptable cost.

---

## Cluster Topology

```
┌──────────────── Home ──────────────────┐       ┌──── AWS (mid-term) ────┐
│  luna              house-of-wind       │       │  quorum-aws            │
│  Framework i5      Ryzen 395 / 3090   │       │  t4g.small (ARM)       │
│  k3s server        k3s server (GPU)    │──TS──▶│  k3s server            │
│  etcd vote 1       etcd vote 2         │       │  etcd vote 3           │
│  CNPG primary      CNPG sync replica   │       │  CNPG async replica    │
└────────────────────────────────────────┘       └────────────────────────┘
                                                  NixOS on EC2, CDK-managed
```

- All nodes connected via **Tailscale mesh** — no public ingress, no VPN tunnels.
- etcd quorum: 3 votes, survives any single node failure.
- GPU workloads (OpenWebUI, LLMs) pinned to house-of-wind.

---

## Storage Layer

### Decision: Local-Path Provisioner (k3s built-in)

**No Longhorn. No Ceph. No Rook.**

Rationale:
- Both local nodes have a single NVMe with everything on root. No dedicated storage disks.
- Two nodes is the worst number for Longhorn replication (no quorum at replica=2).
- With the AWS node, Longhorn would replicate over WAN — slow, expensive, fragile.
- CNPG handles database replication at the application level, which is purpose-built for async geo-replicas and handles latency gracefully.
- Longhorn adds operational complexity (upgrades, CSI quirks, failure modes) with no real benefit over local-path + backups.

### TODO: Dedicated Storage Partition

> [!IMPORTANT]
> Before deploying stateful workloads, carve out a separate LVM logical volume for `/var/lib/rancher/k3s/storage` on both luna and house-of-wind. This prevents PVC data from filling root and bricking the node.

---

## Workload Storage Map

| Service | Database | PVC Storage | Backup Method |
|---------|----------|-------------|---------------|
| **OpenWebUI** | CNPG (Postgres) | local-path (models/config) | CNPG WAL → S3, PVC via Volsync |
| **Home Assistant** | CNPG (Postgres, recorder) | local-path (config) | CNPG WAL → S3, PVC via Volsync |
| **Paperless NGX** | CNPG (Postgres) | local-path (media/documents) | CNPG WAL → S3, **media PVC → S3 (critical!)** |
| **Grafana** | CNPG (Postgres) | — | CNPG WAL → S3, dashboards as code in git |
| **PouchDB (Obsidian)** | CouchDB (not PG) | local-path | Volsync → S3 (small but critical) |
| **Jellyfin** | Config in Helm values | local-path (config), hostPath (media) | Config-as-code. Media: separate concern |
| **Navidrome** | Config in Helm values | local-path (config), hostPath (music) | Config-as-code. Music: separate concern |
| **Prometheus** | None (short retention) | `emptyDir` or short-retention PVC | **No backup.** 24-48h retention for alerting only. Not worth the SSD wear. |
| **SearXNG** | Stateless | — | Nothing to back up |

### Key decisions:
- **All Postgres → CNPG.** One operator, one backup pattern, one replication strategy.
- **PouchDB is the outlier.** CouchDB-compatible, not PG. Gets Volsync/restic treatment.
- **Media (Jellyfin/Navidrome) is a separate concern.** Not backing up by default — it's large and replaceable. Revisit later if desired.
- **Jellyfin & Navidrome config should be fully declarative.** Helm values / configmaps. No manual setup, nothing stateful to lose.

---

## Backup Pipeline

### Database Backups (CNPG)

CNPG handles Postgres backup natively via:
- **Continuous WAL archiving** → S3 (point-in-time recovery)
- **Scheduled `pg_basebackup`** → S3 (full snapshots)

```
CNPG Primary (luna)
  ├─ WAL stream → sync replica (house-of-wind)
  ├─ WAL stream → async replica (quorum-aws)
  ├─ WAL archive → S3 bucket
  └─ Scheduled basebackup → S3 bucket
```

Recovery scenarios:
- **"I dropped a table"** → PITR from WAL archive to any point in time
- **Node dies** → CNPG promotes a replica automatically
- **Both local nodes die** → Promote AWS async replica, minimal data loss (async lag)
- **Total catastrophe** → Restore from S3 basebackup + WAL replay

### PVC Backups (Volsync)

For non-database PVCs (PouchDB, Paperless media, Home Assistant config):
- **Volsync** with restic backend → S3
- Incremental, deduplicated snapshots on a cron schedule
- CRD-based — point it at a PVC, give it an S3 bucket + schedule

### What's NOT backed up
- Prometheus data (intentional — short retention, not worth SSD wear or cost)
- Jellyfin/Navidrome media (large, replaceable — revisit later)
- SearXNG (stateless)
- Any app config that lives in Helm values / git (already backed up by git)

---

## S3 Lifecycle Tiering

Single S3 bucket with lifecycle policy to minimize cost:

| Tier | Object Age | Storage Cost | Retrieval | Purpose |
|------|-----------|-------------|-----------|---------|
| **S3 Standard** | 0–1 day | ~$23/TB/mo | Instant | Active WAL writes, latest basebackup |
| **Glacier Instant Retrieval** | 1–7 days | ~$4/TB/mo | Milliseconds | "Oh shit I deleted that" recovery |
| **Glacier Deep Archive** | 7+ days | ~$1/TB/mo | 12–48 hours | Full cold DR rebuild |

Lifecycle rule:
```json
{
  "Rules": [
    {
      "ID": "homelab-backup-tiering",
      "Status": "Enabled",
      "Filter": { "Prefix": "" },
      "Transitions": [
        { "Days": 1,  "StorageClass": "GLACIER_IR" },
        { "Days": 7,  "StorageClass": "DEEP_ARCHIVE" }
      ],
      "Expiration": { "Days": 90 }
    }
  ]
}
```

> [!NOTE]
> WAL archives may want longer retention than 90 days. Consider a separate prefix/rule for WAL vs basebackup vs PVC snapshots if retention needs diverge.

**Cost model:**
- Homelab PG databases are small — likely <1GB total across all services.
- Paperless media could grow, but deduplication via restic keeps incremental cost low.
- Realistic steady-state: **well under $1/month** for S3 storage.

---

## AWS Quorum Node

### Purpose
- 3rd etcd voter → proper quorum (survives any single node loss)
- CNPG async replica → geo-DR for all Postgres data
- Cheap witness node — not running real workloads

### Implementation: NixOS on EC2 via CDK

**Why NixOS, not a custom AMI:**
- Same config pattern as luna and house-of-wind — just another host in the dotfiles flake
- No Packer, no AMI maintenance, no patching treadmill
- NixOS AMI is just a bootloader; actual system config is in git
- Updates via `nixos-rebuild switch --flake .#quorum-aws`

**Why CDK, not Terraform/raw CloudFormation:**
- Minimal infra to provision (EC2 + SG + EBS, that's it)
- CDK is fine for this scope

**CDK provisions:**
- EC2 instance: `t4g.small` (ARM, ~$12/month)
- Security group: allow Tailscale UDP 41641 inbound, nothing else
- EBS: ~50GB gp3 (etcd + PG replica data)
- User-data: clone flake → `nixos-rebuild switch --flake .#quorum-aws`

**Boot sequence:**
```
CDK deploy
  → EC2 launches with NixOS AMI
  → user-data clones dotfiles flake
  → nixos-rebuild switch --flake .#quorum-aws
  → Tailscale authenticates (auth key from SOPS)
  → k3s server joins via https://luna:6443 over Tailscale
  → CNPG detects new node, begins replica sync
  → done
```

**Dotfiles addition:**
```
dotfiles/
  hosts/
    luna/           ← exists
    house-of-wind/  ← exists
    quorum-aws/     ← new, same pattern
```

### DR: On-Demand Recovery

If the AWS node dies and you're at 2/3 etcd quorum:

1. `cdk deploy` — spins up a fresh replacement (NixOS rebuilds identically from flake)
2. k3s rejoins over Tailscale automatically
3. CNPG resyncs the replica from primary
4. Back to 3/3 quorum

If somehow you need AWS-only quorum (both home nodes down):
- Script spawns a second AWS k3s server from the same CDK/AMI
- Gets you to 2/3 quorum on AWS
- Promote CNPG replica, restore services
- Once local nodes recover, resync and decommission the extra AWS node

### Secrets

- **No IAM Secrets Manager / Parameter Store.** Keep secrets in SOPS (age-encrypted) in the dotfiles repo, same as all other nodes.
- Only AWS-side "secret" needed: a one-time ephemeral Tailscale auth key, passable via CDK parameter or user-data.
- Once Tailscale is up, everything else flows through the mesh using SOPS.

---

## Cost Summary

| Item | Monthly Cost |
|------|-------------|
| AWS EC2 (t4g.small) | ~$12 |
| AWS EBS (50GB gp3) | ~$4 |
| S3 storage (all tiers) | <$1 |
| S3 egress (steady state) | $0 |
| Tailscale (personal plan) | $0 |
| **Total** | **~$17/month** |

DR restore egress (one-time, if needed): varies by data size, but for homelab-scale PG databases it's negligible.

---

## Implementation Order

1. **CNPG operator + first PG cluster** — prove the pattern on one database
2. **S3 bucket + CNPG backup config** — WAL archiving + scheduled basebackups
3. **Volsync** — for non-PG PVCs (PouchDB, Paperless media)
4. **S3 lifecycle policy** — tiered storage (Standard → Glacier IR → Deep Archive)
5. **Dedicated storage partition** — LVM change on luna and house-of-wind (dotfiles)
6. **AWS quorum node** — CDK stack + NixOS host config in dotfiles
7. **DR runbook** — document restore procedures, test them

> [!IMPORTANT]
> Step 1 (CNPG) is the foundation everything else builds on. Start there.
