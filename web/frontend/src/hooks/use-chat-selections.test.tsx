import { renderHook, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { useChatSelections } from "./use-chat-selections"

describe("useChatSelections", () => {
  afterEach(() => vi.unstubAllGlobals())

  it("loads only authoritative targets and routes", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const path = String(input)
      if (path === "/api/provider-targets")
        return Promise.resolve(
          new Response(
            JSON.stringify({ targets: [{ target: "first/model" }] }),
          ),
        )
      if (path === "/api/model-routes")
        return Promise.resolve(
          new Response(
            JSON.stringify({
              routes: [{ name: "primary", targets: ["first/model"] }],
            }),
          ),
        )
      return Promise.resolve(new Response("missing", { status: 404 }))
    })
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useChatSelections(true))
    await waitFor(() => expect(result.current.targets).toHaveLength(1))
    expect(result.current.routes[0]?.name).toBe("primary")
    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/provider-targets",
      "/api/model-routes",
    ])
  })
})
