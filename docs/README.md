# GoSplit documentation

Start with the [project README](../README.md) for quick starts. The pages here go deeper,
one topic each.

## Start here

| If you want to... | Read |
| --- | --- |
| know what the app does | [features.md](features.md) |
| run it for other people | [deployment.md](deployment.md), then [backup-restore.md](backup-restore.md) |
| look up a setting | [configuration.md](configuration.md) |
| understand or change the code | [architecture.md](architecture.md), then [development.md](development.md) |

## All pages

- [features.md](features.md) -- the user guide: friends, groups, expenses, settlements,
  filters, conversion, recurring, notifications, languages, import, bank sync, admin
- [configuration.md](configuration.md) -- every environment variable with its default and
  what it changes; how values are parsed
- [deployment.md](deployment.md) -- the container contract, HTTPS and proxies, Compose and
  plain Docker, PostgreSQL, mail, upgrades, monitoring
- [backup-restore.md](backup-restore.md) -- archives, the four ways to make one, restoring
  safely, startup auto-restore, disaster recovery
- [architecture.md](architecture.md) -- stack, package map, the data model, how the
  frontend stays fresh without a framework, background jobs, the archive format
- [development.md](development.md) -- dev scripts, tests and the coverage floor, the
  knowledge graph, how to extend, versioning and the release pipeline

Design records for past work are not kept here; the decisions that still matter are
folded into the pages above (see [Decisions worth keeping](architecture.md#decisions-worth-keeping)
in architecture.md and [Not supported, by decision](backup-restore.md#not-supported-by-decision)
in backup-restore.md).
