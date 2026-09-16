import { afterEach, describe, expect, it, vi } from "vitest"

import {
  autoConnectFreeProviders,
  createProviderInstance,
  pingProviderInstance,
  syncProviderCatalog,
  updateProviderInstance,
} from "./provider-instances"

describe("provider instance API", () => {
  afterEach(() => vi.unstubAllGlobals())

  it("sends a strict empty catalog sync body", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response("{}", { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await syncProviderCatalog("first")
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/provider-instances/first/catalog/sync",
      expect.objectContaining({ body: "{}", method: "POST" }),
    )
  })

  it("omits write-only values when an edit does not replace them", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response("{}", { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await updateProviderInstance("first", {
      id: "first",
      provider_kind: "openai",
      adapter: "openai-compatible",
      protocol: "openai",
      endpoint: "https://example.test/v1",
      state: "enabled",
    })
    const body = JSON.parse(fetchMock.mock.calls[0][1].body)
    expect(body.write_only).toBeUndefined()
    expect(JSON.stringify(body)).not.toContain("secret")
  })

  it("nests intentionally changed sensitive values under write_only", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response("{}", { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await createProviderInstance({
      id: "first",
      provider_kind: "openai",
      adapter: "openai-compatible",
      protocol: "openai",
      endpoint: "https://example.test/v1",
      state: "enabled",
      write_only: {
        auth_connection_ref: "credential:first",
        headers: { "X-Fixture": "private" },
      },
    })
    const body = JSON.parse(fetchMock.mock.calls[0][1].body)
    expect(body.write_only).toEqual({
      auth_connection_ref: "credential:first",
      headers: { "X-Fixture": "private" },
    })
    expect(body.auth_connection_ref).toBeUndefined()
    expect(body.headers).toBeUndefined()
  })

  it("posts to ping endpoint and auto-connect-free endpoint", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({ ok: true, latency_ms: 42 }), {
            status: 200,
          }),
        ),
      )
    vi.stubGlobal("fetch", fetchMock)
    const pingRes = await pingProviderInstance("groq")
    expect(pingRes.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/provider-instances/groq/ping",
      expect.objectContaining({ method: "POST" }),
    )

    const autoRes = await autoConnectFreeProviders()
    expect(autoRes.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/provider-instances/auto-connect-free",
      expect.objectContaining({ method: "POST" }),
    )
  })
})
