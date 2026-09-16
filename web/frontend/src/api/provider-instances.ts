import { launcherFetch } from "@/api/http"

export type InstanceState = "enabled" | "disabled"

export interface ProviderInstance {
  id: string
  provider_kind: string
  adapter: string
  protocol: string
  endpoint?: string
  auth_configured: boolean
  header_names: string[]
  setting_names: string[]
  state: InstanceState
}

export interface ProviderInstanceInput {
  id: string
  provider_kind: string
  adapter: string
  protocol: string
  endpoint?: string
  state: InstanceState
  write_only?: {
    auth_connection_ref?: string
    headers?: Record<string, string>
    settings?: Record<string, unknown>
  }
  clear_auth_connection?: boolean
  clear_headers?: boolean
  clear_settings?: boolean
}

export interface ProviderTarget {
  target: string
  instance_id: string
  model_id: string
  provider_kind: string
  owned_by?: string
  fetched_at: string
}

export interface ProviderInstanceCatalog {
  instance_id: string
  provider_kind: string
  models: { id: string; owned_by?: string }[]
  fetched_at: string
}

export interface ModelRoute {
  name: string
  targets: string[]
}

export interface ProviderRosterEntry {
  id: string
  display_name: string
  label?: string
  description?: string
  categories?: string[]
  adapter?: string
  protocol?: string
  default_endpoint?: string
  compatibility: "compatible" | "discovery_only" | "native"
  auth_methods?: string[]
  requires_api_key?: boolean
  requires_base_url?: boolean
  anonymous_automation?: boolean
  onboarding_fields?: string[]
  configured?: boolean
  instance_count?: number
  configured_instances?: string[]
}

export interface PingResult {
  ok: boolean
  instance_id: string
  latency_ms: number
  model_count?: number
  status: "reachable" | "unreachable" | "configured"
  error?: string
}

export interface AutoConnectFreeResult {
  ok: boolean
  total: number
  connected: number
  verified: number
  instances: string[]
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await launcherFetch(path, init)
  if (!response.ok)
    throw new Error((await response.text()) || response.statusText)
  return response.json() as Promise<T>
}

const json = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
})

export const listProviderInstances = () =>
  request<{ instances: ProviderInstance[] }>("/api/provider-instances")
export const listProviderRoster = () =>
  request<{ providers: ProviderRosterEntry[] }>("/api/provider-roster")
export const createProviderInstance = (body: ProviderInstanceInput) =>
  request("/api/provider-instances", json(body))
export const updateProviderInstance = (
  id: string,
  body: ProviderInstanceInput,
) =>
  request(`/api/provider-instances/${encodeURIComponent(id)}`, {
    ...json(body),
    method: "PUT",
  })
export const deleteProviderInstance = (id: string) =>
  request(`/api/provider-instances/${encodeURIComponent(id)}`, {
    method: "DELETE",
  })
export const syncProviderCatalog = (id: string) =>
  request(
    `/api/provider-instances/${encodeURIComponent(id)}/catalog/sync`,
    json({}),
  )
export const pingProviderInstance = (id: string) =>
  request<PingResult>(
    `/api/provider-instances/${encodeURIComponent(id)}/ping`,
    json({}),
  )
export const autoConnectFreeProviders = () =>
  request<AutoConnectFreeResult>(
    "/api/provider-instances/auto-connect-free",
    json({}),
  )
export const getActiveModels = () =>
  request<{ active_models: string[]; total: number }>("/api/active-models")
export const setActiveModels = (models: string[]) =>
  request<{ active_models: string[]; status: string }>(
    "/api/active-models",
    json({ models }),
  )
export const addActiveModel = (target: string) =>
  request<{ active_models: string[]; status: string }>(
    "/api/active-models/add",
    json({ target }),
  )
export const removeActiveModel = (target: string) =>
  request<{ active_models: string[]; status: string }>(
    "/api/active-models/remove",
    json({ target }),
  )
export const listProviderTargets = () =>
  request<{ targets: ProviderTarget[] }>("/api/provider-targets")
export const listProviderInstanceCatalogs = () =>
  request<{ catalogs: ProviderInstanceCatalog[] }>(
    "/api/provider-instances/catalogs",
  )
export const listModelRoutes = () =>
  request<{ routes: ModelRoute[] }>("/api/model-routes")
export const createModelRoute = (route: ModelRoute) =>
  request("/api/model-routes", json(route))
export const updateModelRoute = (name: string, route: ModelRoute) =>
  request(`/api/model-routes/${encodeURIComponent(name)}`, {
    ...json(route),
    method: "PUT",
  })
export const deleteModelRoute = (name: string) =>
  request(`/api/model-routes/${encodeURIComponent(name)}`, { method: "DELETE" })
