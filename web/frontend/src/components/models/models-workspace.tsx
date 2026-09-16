import {
  IconCheck,
  IconKey,
  IconPlug,
  IconPlus,
  IconRefresh,
  IconRoute,
  IconSearch,
  IconSparkles,
  IconTrash,
  IconX,
} from "@tabler/icons-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type ModelRoute,
  type PingResult,
  type ProviderInstance,
  type ProviderInstanceInput,
  type ProviderRosterEntry,
  type ProviderTarget,
  addActiveModel,
  autoConnectFreeProviders,
  createModelRoute,
  createProviderInstance,
  deleteModelRoute,
  deleteProviderInstance,
  getActiveModels,
  pingProviderInstance,
  removeActiveModel,
  syncProviderCatalog,
  updateModelRoute,
  updateProviderInstance,
} from "@/api/provider-instances"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useProviderInstances } from "@/hooks/use-provider-instances"

import { LegacyModelsPage } from "./models-page"
import { ProviderIcon } from "./provider-icon"

type Section = "instances" | "routes" | "legacy"

export function ModelsWorkspace() {
  const { t } = useTranslation()
  const [section, setSection] = useState<Section>("instances")
  const state = useProviderInstances()
  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.models")} />
      <div
        className="border-border/60 flex gap-1 border-b px-4 sm:px-6"
        role="tablist"
      >
        {(["instances", "routes", "legacy"] as const).map((item) => (
          <Button
            key={item}
            role="tab"
            aria-selected={section === item}
            variant="ghost"
            className={`rounded-none border-x-0 border-t-0 ${section === item ? "bg-muted text-foreground shadow-[inset_0_-1px_0_hsl(var(--foreground))]" : "text-muted-foreground border-transparent"}`}
            onClick={() => setSection(item)}
          >
            {t(`models.management.tabs.${item}`)}
          </Button>
        ))}
      </div>
      {section === "legacy" ? (
        <div className="min-h-0 flex-1" role="tabpanel">
          <LegacyModelsPage embedded />
        </div>
      ) : (
        <div
          className="min-h-0 flex-1 overflow-y-auto px-4 py-6 sm:px-6"
          role="tabpanel"
        >
          <div className="mx-auto max-w-6xl">
            {state.error && (
              <p className="text-destructive mb-4 text-sm">{state.error}</p>
            )}
            {state.loading ? (
              <p className="text-muted-foreground text-sm">
                {t("common.loading")}
              </p>
            ) : section === "instances" ? (
              <UnifiedProviderHub {...state} />
            ) : (
              <RoutesPanel {...state} />
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function UnifiedProviderHub({
  instances,
  catalogs,
  targets,
  roster,
  refresh,
}: ReturnType<typeof useProviderInstances>) {
  const [search, setSearch] = useState("")
  const [filter, setFilter] = useState("all")
  const [expandedId, setExpandedId] = useState("")
  const [pingingId, setPingingId] = useState<string | null>(null)
  const [pingResults, setPingResults] = useState<Record<string, PingResult>>({})
  const [autoConnecting, setAutoConnecting] = useState(false)
  const [activeModels, setActiveModelsState] = useState<string[]>([])
  const [editing, setEditing] = useState<
    ProviderInstance | ProviderRosterEntry | null
  >(null)

  const loadActive = async () => {
    try {
      const res = await getActiveModels()
      setActiveModelsState(res.active_models || [])
    } catch {
      setActiveModelsState([])
    }
  }

  useEffect(() => {
    void loadActive()
  }, [])

  const mutate = async (operation: () => Promise<unknown>) => {
    try {
      await operation()
      await refresh()
      await loadActive()
      return true
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : "Request failed")
      return false
    }
  }

  const handlePing = async (instanceId: string) => {
    setPingingId(instanceId)
    try {
      const res = await pingProviderInstance(instanceId)
      setPingResults((prev) => ({ ...prev, [instanceId]: res }))
      if (res.ok) {
        toast.success(`Ping successful: ${res.latency_ms}ms (${res.model_count ?? 0} models)`)
      } else {
        toast.error(`Ping failed: ${res.error || "Unreachable"}`)
      }
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : "Ping request failed")
    } finally {
      setPingingId(null)
    }
  }

  const handleAutoConnectFree = async () => {
    setAutoConnecting(true)
    try {
      const res = await autoConnectFreeProviders()
      toast.success(
        `Auto-connected ${res.connected} free zero-key providers (${res.verified} verified active)!`,
      )
      await refresh()
      await loadActive()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : "Auto-connect failed")
    } finally {
      setAutoConnecting(false)
    }
  }

  const handleToggleShortlist = async (target: string) => {
    const isShortlisted = activeModels.includes(target)
    if (isShortlisted) {
      await mutate(() => removeActiveModel(target))
      toast.info(`Removed ${target} from Chat shortlist`)
    } else {
      await mutate(() => addActiveModel(target))
      toast.success(`Added ${target} to Chat shortlist`)
    }
  }

  const allDisplayProviders = useMemo(() => {
    const list: ProviderRosterEntry[] = [...roster]
    const rosterIDs = new Set(roster.map((r) => r.id))
    for (const inst of instances) {
      if (!rosterIDs.has(inst.provider_kind) && !rosterIDs.has(inst.id)) {
        list.push({
          id: inst.id,
          display_name: inst.id,
          adapter: inst.adapter,
          protocol: inst.protocol,
          default_endpoint: inst.endpoint,
          compatibility: "compatible",
          configured: true,
          instance_count: 1,
          configured_instances: [inst.id],
        })
      }
    }
    return list
  }, [roster, instances])

  const filteredProviders = useMemo(() => {
    return allDisplayProviders.filter((entry) => {
      const q = search.trim().toLowerCase()
      if (q) {
        const match =
          entry.id.toLowerCase().includes(q) ||
          entry.display_name.toLowerCase().includes(q) ||
          (entry.description && entry.description.toLowerCase().includes(q)) ||
          (entry.protocol && entry.protocol.toLowerCase().includes(q))
        if (!match) return false
      }

      const matchingInsts = instances.filter(
        (i) => i.provider_kind === entry.id || i.id === entry.id,
      )
      const isConfigured = matchingInsts.length > 0 || entry.configured

      if (filter === "configured") return isConfigured
      if (filter === "device")
        return (
          entry.id.includes("copilot") ||
          entry.id.includes("codex") ||
          entry.auth_methods?.includes("oauth_device") ||
          entry.compatibility === "native"
        )
      if (filter === "free") return Boolean(entry.anonymous_automation)
      if (filter === "local")
        return (
          entry.id.includes("ollama") ||
          entry.id.includes("lmstudio") ||
          entry.id.includes("local")
        )
      if (filter === "apikey")
        return (
          entry.requires_api_key &&
          !entry.id.includes("copilot") &&
          !entry.id.includes("codex")
        )
      return true
    })
  }, [allDisplayProviders, instances, search, filter])

  const shelves = useMemo(() => {
    const groups = [
      {
        key: "device",
        title: "Device & Subscription Sign-In",
        description: "Official device authentication leveraging personal or enterprise subscriptions.",
        entries: filteredProviders.filter(
          (p) =>
            p.id.includes("copilot") ||
            p.id.includes("codex") ||
            p.auth_methods?.includes("oauth_device") ||
            p.compatibility === "native",
        ),
      },
      {
        key: "free",
        title: "Free & Anonymous (Zero Key)",
        description: "Open access models that require no API key or subscription.",
        entries: filteredProviders.filter(
          (p) =>
            p.anonymous_automation ||
            p.id.includes("zen") ||
            p.id.includes("pollinations") ||
            p.id.includes("kilo"),
        ),
      },
      {
        key: "apikey",
        title: "API Key Providers",
        description: "Standard model endpoints authenticated via your provider API key.",
        entries: filteredProviders.filter(
          (p) =>
            !p.id.includes("copilot") &&
            !p.id.includes("codex") &&
            !p.anonymous_automation &&
            !p.id.includes("ollama") &&
            !p.id.includes("lmstudio") &&
            !p.id.includes("local") &&
            (p.adapter === "openai-compatible" ||
              p.adapter === "anthropic-compatible" ||
              p.requires_api_key),
        ),
      },
      {
        key: "local",
        title: "Local & Self-Hosted",
        description: "In-network and local models running on your own hardware.",
        entries: filteredProviders.filter(
          (p) =>
            p.id.includes("ollama") ||
            p.id.includes("lmstudio") ||
            p.id.includes("local"),
        ),
      },
    ]

    const groupedIds = new Set(
      groups.flatMap((g) => g.entries.map((e) => e.id)),
    )
    const otherEntries = filteredProviders.filter((p) => !groupedIds.has(p.id))
    if (otherEntries.length > 0) {
      groups.push({
        key: "other",
        title: "Other candidates",
        description: "Additional supported and discovered model integrations.",
        entries: otherEntries,
      })
    }

    return groups.filter((g) => g.entries.length > 0)
  }, [filteredProviders])

  return (
    <div className="space-y-8">
      {/* 1. Active Chat Models Shortlist */}
      <section className="rounded-xl border border-border bg-card p-5 shadow-xs">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-4">
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-base font-semibold text-foreground">
                Active Chat Models (Shortlist)
              </h3>
              <Badge variant="secondary" className="text-xs">
                {activeModels.length} in Chat selector
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              Only models on this shortlist appear in the Chat dropdown. Add models from any connected provider below.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={autoConnecting}
              onClick={handleAutoConnectFree}
            >
              <IconSparkles
                className={`size-4 text-amber-500 mr-1.5 ${autoConnecting ? "animate-spin" : ""}`}
              />
              {autoConnecting ? "Connecting..." : "Auto-connect Free"}
            </Button>
            <Button
              size="sm"
              onClick={() =>
                setEditing({
                  id: "custom",
                  display_name: "Custom OpenAI-compatible",
                  adapter: "openai-compatible",
                  protocol: "openai",
                  compatibility: "compatible",
                })
              }
            >
              <IconPlus className="size-4 mr-1.5" />
              + Custom
            </Button>
          </div>
        </div>

        {activeModels.length > 0 ? (
          <div className="divide-y divide-border/50 overflow-hidden rounded-lg border border-border/80 bg-background/50">
            {activeModels.map((target, idx) => {
              const [instId, modelId] = target.split("/")
              const isCopilot = instId.includes("copilot")
              const isFree = instId.includes("zen") || instId.includes("pollinations") || instId.includes("kilo")
              return (
                <div
                  key={target}
                  className="flex items-center justify-between p-3 gap-3"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <ProviderIcon provider={{ key: instId }} className="size-5" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <strong className="text-sm font-medium text-foreground truncate">
                          {modelId || target}
                        </strong>
                        <span className="text-xs text-muted-foreground">
                          ({instId})
                        </span>
                        {idx === 0 && (
                          <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary">
                            Default
                          </span>
                        )}
                        {isCopilot && (
                          <span className="rounded bg-purple-500/10 px-1.5 py-0.5 text-[10px] font-medium text-purple-600 dark:text-purple-400">
                            Device Auth
                          </span>
                        )}
                        {isFree && (
                          <span className="rounded bg-sky-500/10 px-1.5 py-0.5 text-[10px] font-medium text-sky-600 dark:text-sky-400">
                            Zero Key
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 px-2 text-xs text-destructive hover:bg-destructive/10"
                      onClick={() => void handleToggleShortlist(target)}
                    >
                      <IconTrash className="size-3.5 mr-1" />
                      Remove from Chat
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-border p-6 text-center">
            <p className="text-sm font-medium text-foreground">
              No models in Chat shortlist
            </p>
            <p className="text-xs text-muted-foreground mt-1 max-w-md mx-auto">
              Click <strong>&quot;Auto-connect Free&quot;</strong> above to instantly add verified free models, or expand any provider below to add specific models to Chat.
            </p>
          </div>
        )}
      </section>

      {/* 2. Provider Hub Header & Search */}
      <section className="space-y-4">
        <div>
          <h3 className="text-xl font-bold tracking-tight text-foreground">
            Providers
          </h3>
          <p className="text-sm text-muted-foreground mt-0.5">
            Configure providers, check connection health, and browse discovered models.
          </p>
        </div>

        <div className="flex flex-col gap-3">
          <div className="relative">
            <IconSearch className="text-muted-foreground absolute left-3 top-1/2 size-4 -translate-y-1/2" />
            <Input
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setExpandedId("")
              }}
              placeholder="Search name, API or instance ID"
              className="pl-9"
            />
          </div>
          <div
            className="flex flex-wrap gap-1.5"
            role="group"
            aria-label="Provider filters"
          >
            {[
              { key: "all", label: `All providers (${allDisplayProviders.length})` },
              { key: "configured", label: `Your accounts (${instances.length})` },
              { key: "device", label: "Device sign-in" },
              { key: "free", label: "Free · no key" },
              { key: "apikey", label: "API key" },
              { key: "local", label: "Local" },
            ].map((chip) => (
              <button
                key={chip.key}
                type="button"
                onClick={() => {
                  setFilter(chip.key)
                  setExpandedId("")
                }}
                className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                  filter === chip.key
                    ? "bg-primary text-primary-foreground shadow-xs"
                    : "bg-muted text-muted-foreground hover:bg-muted/80 hover:text-foreground"
                }`}
              >
                {chip.label}
              </button>
            ))}
          </div>
        </div>

        {/* 3. Shelves */}
        <div className="flex flex-col gap-8 pt-2">
          {shelves.map((shelf) => {
            const expandedEntry = shelf.entries.find((e) => e.id === expandedId)
            return (
              <section key={shelf.key} className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <h4 className="text-sm font-semibold tracking-wide text-foreground">
                      {shelf.title}
                    </h4>
                    <span className="text-muted-foreground text-xs font-normal">
                      {shelf.entries.length}
                    </span>
                  </div>
                  <span className="text-xs text-muted-foreground hidden sm:inline">
                    {shelf.description}
                  </span>
                </div>

                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                  {shelf.entries.map((entry) => {
                    const isExpanded = expandedId === entry.id
                    const matchingInsts = instances.filter(
                      (inst) =>
                        inst.provider_kind === entry.id || inst.id === entry.id,
                    )
                    const isConfigured =
                      matchingInsts.length > 0 || Boolean(entry.configured)
                    return (
                      <button
                        key={entry.id}
                        type="button"
                        onClick={() => setExpandedId(isExpanded ? "" : entry.id)}
                        className={`flex items-center gap-2.5 rounded-lg border p-3 text-left transition-all hover:bg-muted/50 ${
                          isExpanded
                            ? "border-primary bg-primary/5 ring-1 ring-primary"
                            : "border-border bg-card"
                        }`}
                      >
                        <ProviderIcon provider={{ key: entry.id, label: entry.display_name }} />
                        <span className="truncate text-xs font-medium text-foreground flex-1">
                          {entry.display_name}
                        </span>
                        {isConfigured && (
                          <span
                            className="size-2 rounded-full bg-emerald-500 shrink-0"
                            title="Instance configured"
                          />
                        )}
                      </button>
                    )
                  })}
                </div>

                {expandedEntry && (
                  <ExpandedInspector
                    entry={expandedEntry}
                    instances={instances.filter(
                      (i) =>
                        i.provider_kind === expandedEntry.id ||
                        i.id === expandedEntry.id,
                    )}
                    catalogs={catalogs}
                    targets={targets}
                    activeModels={activeModels}
                    onToggleShortlist={handleToggleShortlist}
                    pingResult={
                      pingResults[expandedEntry.id] ||
                      pingResults[
                        instances.find(
                          (i) =>
                            i.provider_kind === expandedEntry.id ||
                            i.id === expandedEntry.id,
                        )?.id || ""
                      ]
                    }
                    isPinging={
                      pingingId === expandedEntry.id ||
                      pingingId ===
                        instances.find(
                          (i) =>
                            i.provider_kind === expandedEntry.id ||
                            i.id === expandedEntry.id,
                        )?.id
                    }
                    onClose={() => setExpandedId("")}
                    onConfigure={() => setEditing(expandedEntry)}
                    onEditInstance={(inst) => setEditing(inst)}
                    onPing={(id) => handlePing(id)}
                    onSync={(id) => void mutate(() => syncProviderCatalog(id))}
                    onDelete={(id) =>
                      void mutate(() => deleteProviderInstance(id))
                    }
                  />
                )}
              </section>
            )
          })}

          {shelves.length === 0 && (
            <div className="rounded-xl border border-dashed p-12 text-center">
              <p className="text-sm font-medium text-foreground">
                No matching providers found
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Adjust your search or filter to inspect another provider.
              </p>
            </div>
          )}
        </div>
      </section>

      {editing && (
        <InstanceDialog
          value={editing}
          onClose={() => setEditing(null)}
          onSave={async (body, existing) => {
            const saved = await mutate(() =>
              existing
                ? updateProviderInstance(existing, body)
                : createProviderInstance(body),
            )
            if (saved) setEditing(null)
          }}
        />
      )}
    </div>
  )
}

function ExpandedInspector({
  entry,
  instances,
  catalogs,
  activeModels,
  onToggleShortlist,
  pingResult,
  isPinging,
  onClose,
  onConfigure,
  onEditInstance,
  onPing,
  onSync,
  onDelete,
}: {
  entry: ProviderRosterEntry
  instances: ProviderInstance[]
  catalogs: ReturnType<typeof useProviderInstances>["catalogs"]
  targets: ReturnType<typeof useProviderInstances>["targets"]
  activeModels: string[]
  onToggleShortlist: (target: string) => Promise<void>
  pingResult?: PingResult
  isPinging: boolean
  onClose: () => void
  onConfigure: () => void
  onEditInstance: (inst: ProviderInstance) => void
  onPing: (id: string) => void
  onSync: (id: string) => void
  onDelete: (id: string) => void
}) {
  const activeInstance = instances[0]
  const catalog = activeInstance
    ? catalogs.find((c) => c.instance_id === activeInstance.id)
    : null
  const isConfigured = Boolean(activeInstance)
  const isDiscoveryOnly = entry.compatibility === "discovery_only"

  return (
    <div className="mt-3 rounded-xl border border-border bg-card p-5 shadow-xs">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <ProviderIcon provider={{ key: entry.id, label: entry.display_name }} className="size-6" />
          <div>
            <div className="flex items-center gap-2">
              <h4 className="text-base font-semibold text-foreground">
                {entry.display_name}
              </h4>
              {catalog && catalog.models.length > 0 && (
                <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                  Catalog Synced
                </span>
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {isConfigured
                ? `${instances.length} instance${instances.length > 1 ? "s" : ""} configured`
                : "Available provider integration"}
            </p>
          </div>
        </div>
        <Button
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={onClose}
          aria-label="Close"
        >
          <IconX className="size-4" />
        </Button>
      </div>

      {entry.description && (
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed max-w-3xl">
          {entry.description}
        </p>
      )}

      <div className="mt-4 grid grid-cols-2 gap-4 border-y border-border/60 py-3 text-xs sm:grid-cols-4">
        <div>
          <span className="text-muted-foreground block font-medium">Catalog</span>
          <span className="mt-1 block font-semibold text-foreground">
            {catalog?.models.length
              ? `${catalog.models.length} models · fresh`
              : "Not synced"}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground block font-medium">Protocol</span>
          <span className="mt-1 block font-semibold text-foreground">
            {entry.protocol || entry.adapter || "openai"}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground block font-medium">Auth</span>
          <span className="mt-1 block font-semibold text-foreground">
            {entry.auth_methods?.join(", ") ||
              (entry.requires_api_key ? "API key" : "none")}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground block font-medium">Category</span>
          <span className="mt-1 block font-semibold text-foreground">
            {entry.compatibility === "native"
              ? "Device Auth"
              : entry.anonymous_automation
                ? "Free Tier"
                : "API Key"}
          </span>
        </div>
        <div className="col-span-2 sm:col-span-4">
          <span className="text-muted-foreground block font-medium">Endpoint</span>
          <span className="mt-1 block font-mono text-[11px] text-muted-foreground">
            {activeInstance?.endpoint ||
              entry.default_endpoint ||
              "Dynamic / In-process runtime"}
          </span>
        </div>
      </div>

      {/* Discovered models list with "+ Add to Chat" */}
      {catalog && catalog.models.length > 0 && (
        <div className="mt-4 pt-2">
          <span className="text-xs font-semibold text-foreground mb-2 block">
            Discovered Models ({catalog.models.length})
          </span>
          <div className="max-h-48 overflow-y-auto rounded-lg border border-border divide-y divide-border/60 bg-muted/20">
            {catalog.models.map((m) => {
              const exactTarget = `${activeInstance.id}/${m.id}`
              const inShortlist = activeModels.includes(exactTarget)
              return (
                <div
                  key={m.id}
                  className="flex items-center justify-between p-2 px-3 text-xs"
                >
                  <span className="font-mono text-foreground font-medium truncate max-w-md">
                    {m.id}
                  </span>
                  <Button
                    size="sm"
                    variant={inShortlist ? "outline" : "secondary"}
                    className="h-6 px-2 text-[11px]"
                    onClick={() => void onToggleShortlist(exactTarget)}
                  >
                    {inShortlist ? (
                      <>
                        <IconCheck className="size-3 text-emerald-500 mr-1" />
                        In Chat
                      </>
                    ) : (
                      <>
                        <IconPlus className="size-3 mr-1" />
                        Add to Chat
                      </>
                    )}
                  </Button>
                </div>
              )
            })}
          </div>
        </div>
      )}

      <div className="mt-5 flex flex-wrap items-center gap-2">
        {!isConfigured ? (
          !isDiscoveryOnly && (
            <Button size="sm" onClick={onConfigure}>
              <IconPlus className="size-4 mr-1" />
              Connect provider
            </Button>
          )
        ) : (
          <>
            <Button size="sm" onClick={onConfigure}>
              <IconPlus className="size-4 mr-1" />
              Add instance
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={isPinging}
              onClick={() => onPing(activeInstance.id)}
            >
              <IconPlug className="size-4 mr-1" />
              {isPinging ? "Checking..." : "Check reachability"}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => onSync(activeInstance.id)}
            >
              <IconRefresh className="size-4 mr-1" />
              Sync catalog
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => onEditInstance(activeInstance)}
            >
              Edit
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="text-destructive hover:bg-destructive/10"
              onClick={() => onDelete(activeInstance.id)}
            >
              <IconTrash className="size-4 mr-1" />
              Remove
            </Button>
            {pingResult && (
              <span
                className={`rounded-md px-2.5 py-1 text-xs font-medium ${
                  pingResult.ok
                    ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                    : "bg-destructive/10 text-destructive"
                }`}
              >
                {pingResult.ok
                  ? `✓ ${pingResult.latency_ms}ms reachable`
                  : `✗ ${pingResult.error || "Unreachable"}`}
              </span>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function InstanceDialog({
  value,
  onClose,
  onSave,
}: {
  value: ProviderInstance | ProviderRosterEntry
  onClose: () => void
  onSave: (body: ProviderInstanceInput, existing?: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const existing = value && "auth_configured" in value ? value : null
  const roster = value && "compatibility" in value ? value : null
  const [id, setID] = useState(existing?.id || roster?.id || "")
  const providerKind = existing?.provider_kind || roster?.id || ""
  const adapter = existing?.adapter || roster?.adapter || "openai-compatible"
  const protocol = existing?.protocol || roster?.protocol || "openai"
  const [endpoint, setEndpoint] = useState(
    existing?.endpoint || roster?.default_endpoint || "",
  )
  const state: "enabled" | "disabled" = existing?.state || "enabled"
  const [authRef, setAuthRef] = useState("")
  const headers = ""
  const settings = ""
  const clearAuth = false
  const clearHeaders = false
  const clearSettings = false
  const [formError, setFormError] = useState("")

  const isCopilot =
    id.includes("copilot") ||
    providerKind.includes("copilot") ||
    adapter === "github-copilot-native"

  const submit = async () => {
    try {
      const body: ProviderInstanceInput = {
        id,
        provider_kind: providerKind,
        adapter,
        protocol,
        endpoint,
        state,
      }
      const writeOnly: NonNullable<ProviderInstanceInput["write_only"]> = {}
      if (authRef.trim()) writeOnly.auth_connection_ref = authRef.trim()
      if (headers.trim()) {
        const parsed = JSON.parse(headers)
        if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
          throw new Error("Headers must be a JSON object")
        }
        writeOnly.headers = parsed
      }
      if (settings.trim()) {
        const parsed = JSON.parse(settings)
        if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
          throw new Error("Settings must be a JSON object")
        }
        writeOnly.settings = parsed
      }
      if (Object.keys(writeOnly).length > 0) body.write_only = writeOnly
      if (clearAuth) body.clear_auth_connection = true
      if (clearHeaders) body.clear_headers = true
      if (clearSettings) body.clear_settings = true
      await onSave(body, existing?.id)
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : "Validation failed")
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <div className="flex items-center gap-2 mb-1">
            <ProviderIcon provider={{ key: providerKind || id }} className="size-5" />
            <DialogTitle>
              {existing
                ? t("models.management.instances.editTitle")
                : `Connect ${roster?.display_name || "Provider"}`}
            </DialogTitle>
          </div>
          <p className="text-xs text-muted-foreground">
            {isCopilot
              ? "Uses your GitHub Copilot subscription. Once authorized, Studio exchanges session tokens with the official Copilot endpoint."
              : "Credential input is write-only. Secrets are stored securely and never displayed."}
          </p>
        </DialogHeader>

        {formError && <p className="text-destructive text-sm">{formError}</p>}

        <div className="space-y-3 py-2 text-xs">
          <div>
            <label className="text-foreground block font-medium mb-1">
              Provider Instance ID
            </label>
            <Input
              value={id}
              disabled={Boolean(existing)}
              onChange={(e) => setID(e.target.value)}
              placeholder="e.g. deepseek, groq"
            />
          </div>

          {!isCopilot && (
            <>
              <div>
                <label className="text-foreground block font-medium mb-1">
                  API Key or Secret Token
                </label>
                <Input
                  type="password"
                  value={authRef}
                  onChange={(e) => setAuthRef(e.target.value)}
                  placeholder={existing ? "(Leave blank to keep current key)" : "sk-..."}
                />
              </div>

              <div>
                <label className="text-foreground block font-medium mb-1">
                  Base URL / Endpoint
                </label>
                <Input
                  value={endpoint}
                  onChange={(e) => setEndpoint(e.target.value)}
                  placeholder="https://api.example.com/v1"
                />
              </div>
            </>
          )}

          {isCopilot && (
            <div className="rounded-lg border border-purple-500/30 bg-purple-500/10 p-3 text-xs text-purple-700 dark:text-purple-300">
              <div className="flex items-center gap-1.5 font-semibold mb-1">
                <IconKey className="size-4" />
                Device Authentication Active
              </div>
              <p className="text-[11px] leading-relaxed">
                Connects through your GitHub OAuth authorization token and auto-refreshes Copilot session tokens dynamically (supporting individual, business, and enterprise accounts).
              </p>
            </div>
          )}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={submit}>
            {existing ? "Save changes" : "Connect provider"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function RoutesPanel({
  targets,
  routes,
  refresh,
}: ReturnType<typeof useProviderInstances>) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<ModelRoute | null>(null)
  const [creating, setCreating] = useState(false)
  const mutate = async (operation: () => Promise<unknown>) => {
    try {
      await operation()
      await refresh()
      return true
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : "Request failed")
      return false
    }
  }
  return (
    <>
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">
            {t("models.management.routes.title")}
          </h2>
          <p className="text-muted-foreground mt-1 max-w-2xl text-sm">
            {t("models.management.routes.description")}
          </p>
        </div>
        <Button size="sm" onClick={() => setCreating(true)}>
          <IconRoute className="size-4" />
          {t("models.management.routes.add")}
        </Button>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <h3 className="mb-3 font-medium">
            {t("models.management.routes.listTitle")}
          </h3>
          <div className="divide-border overflow-hidden rounded-xl border">
            {routes.map((route) => (
              <div
                key={route.name}
                className="flex items-center justify-between p-4"
              >
                <div>
                  <strong>{route.name}</strong>
                  <p className="text-muted-foreground mt-1 text-xs">
                    {t("models.management.routes.orderedTargets", {
                      count: route.targets.length,
                    })}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setEditing(route)}
                  >
                    {t("models.management.actions.edit")}
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    aria-label={t("models.management.actions.deleteRoute", {
                      name: route.name,
                    })}
                    onClick={() =>
                      void mutate(() => deleteModelRoute(route.name))
                    }
                  >
                    <IconTrash className="size-4" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
        <div>
          <h3 className="mb-3 font-medium">
            {t("models.management.routes.verifiedTitle")}
          </h3>
          <div className="divide-border max-h-96 overflow-y-auto rounded-xl border">
            {targets.map((target) => (
              <div
                key={target.target}
                className="flex items-center justify-between p-3 text-sm"
              >
                <span className="font-mono text-xs">{target.target}</span>
                <span className="text-muted-foreground text-xs">
                  {target.provider_kind}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
      {(creating || editing) && (
        <RouteDialog
          route={editing}
          targets={targets}
          onClose={() => {
            setEditing(null)
            setCreating(false)
          }}
          onSave={async (saved) => {
            const ok = await mutate(() =>
              editing
                ? updateModelRoute(editing.name, saved)
                : createModelRoute(saved),
            )
            if (ok) {
              setEditing(null)
              setCreating(false)
            }
          }}
        />
      )}
    </>
  )
}

function RouteDialog({
  route,
  targets,
  onClose,
  onSave,
}: {
  route: ModelRoute | null
  targets: ProviderTarget[]
  onClose: () => void
  onSave: (route: ModelRoute) => Promise<void>
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(route?.name || "")
  const [selected, setSelected] = useState<string[]>(route?.targets || [])
  const [candidate, setCandidate] = useState(targets[0]?.target || "")
  const available = targets.filter(
    (target) => !selected.includes(target.target),
  )

  const move = (index: number, delta: number) => {
    const next = [...selected]
    const item = next[index]
    next[index] = next[index + delta]
    next[index + delta] = item
    setSelected(next)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {route
              ? t("models.management.routes.editTitle")
              : t("models.management.routes.createTitle")}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <Input
            value={name}
            disabled={Boolean(route)}
            onChange={(e) => setName(e.target.value)}
            placeholder="Route name (e.g. primary)"
          />
          <div className="space-y-2">
            {selected.map((item, index) => (
              <div
                key={item}
                className="flex items-center justify-between rounded-lg border p-2 text-sm"
              >
                <span className="font-mono text-xs">{item}</span>
                <div className="flex gap-1">
                  <Button
                    size="xs"
                    variant="ghost"
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    {t("models.management.routes.up")}
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    disabled={index === selected.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    {t("models.management.routes.down")}
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    onClick={() =>
                      setSelected(selected.filter((entry) => entry !== item))
                    }
                  >
                    <IconTrash className="size-3" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {available.length > 0 && (
            <div className="flex gap-2">
              <select
                className="border-input bg-background flex-1 rounded-md border px-3 py-1 text-sm"
                value={candidate}
                onChange={(e) => setCandidate(e.target.value)}
              >
                {available.map((item) => (
                  <option key={item.target} value={item.target}>
                    {item.target}
                  </option>
                ))}
              </select>
              <Button
                size="sm"
                onClick={() => {
                  setSelected([...selected, candidate])
                  const remaining = available.filter(
                    (item) => item.target !== candidate,
                  )
                  setCandidate(remaining[0]?.target || "")
                }}
              >
                Add
              </Button>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={() => onSave({ name, targets: selected })}
            disabled={!name.trim() || selected.length === 0}
          >
            {t("models.management.routes.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
