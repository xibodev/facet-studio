import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { SidebarProvider } from "@/components/ui/sidebar"
import "@/i18n"

import { ModelsWorkspace } from "./models-workspace"

const response = (value: unknown) =>
  Promise.resolve(new Response(JSON.stringify(value), { status: 200 }))

describe("ModelsWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const path = String(input)
        if (path === "/api/provider-instances")
          return response({
            instances: [
              {
                id: "first",
                provider_kind: "openai",
                adapter: "openai-compatible",
                protocol: "openai",
                endpoint: "https://first.test/v1",
                auth_configured: true,
                header_names: [],
                setting_names: [],
                state: "enabled",
              },
              {
                id: "second",
                provider_kind: "openai",
                adapter: "openai-compatible",
                protocol: "openai",
                endpoint: "https://second.test/v1",
                auth_configured: true,
                header_names: [],
                setting_names: [],
                state: "enabled",
              },
            ],
          })
        if (path === "/api/provider-targets")
          return response({
            targets: [
              {
                target: "first/shared",
                instance_id: "first",
                model_id: "shared",
                provider_kind: "openai",
                fetched_at: "",
              },
              {
                target: "second/shared",
                instance_id: "second",
                model_id: "shared",
                provider_kind: "openai",
                fetched_at: "",
              },
            ],
          })
        if (path === "/api/provider-instances/catalogs")
          return response({
            catalogs: [
              {
                instance_id: "first",
                provider_kind: "openai",
                models: [{ id: "shared" }],
                fetched_at: "",
              },
              {
                instance_id: "second",
                provider_kind: "openai",
                models: [{ id: "shared" }],
                fetched_at: "",
              },
            ],
          })
        if (path === "/api/model-routes")
          return response({
            routes: [
              { name: "primary", targets: ["second/shared", "first/shared"] },
            ],
          })
        if (path === "/api/provider-roster")
          return response({
            providers: [
              {
                id: "unknown",
                display_name: "Unknown protocol",
                compatibility: "discovery_only",
              },
            ],
          })
        return response({})
      }),
    )
  })

  const renderWorkspace = () =>
    render(
      <SidebarProvider>
        <ModelsWorkspace />
      </SidebarProvider>,
    )

  it("shows discovery-only providers without a configure action", async () => {
    renderWorkspace()
    expect(await screen.findByText("Unknown protocol")).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: "Configure" }),
    ).not.toBeInTheDocument()
  })

  it("keeps identical model IDs distinct by exact instance target", async () => {
    renderWorkspace()
    await userEvent.click(
      await screen.findByRole("tab", { name: "Models & Routes" }),
    )
    expect(screen.getAllByText("first/shared").length).toBeGreaterThan(0)
    expect(screen.getAllByText("second/shared").length).toBeGreaterThan(0)
    expect(screen.queryByText("gpt-5.4-mini")).not.toBeInTheDocument()
  })

  it("edits route order using only backend-provided targets", async () => {
    renderWorkspace()
    await userEvent.click(
      await screen.findByRole("tab", { name: "Models & Routes" }),
    )
    await userEvent.click(screen.getByRole("button", { name: "Edit" }))
    expect(await screen.findByText("Edit route")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getAllByText("second/shared").length).toBeGreaterThan(1),
    )
  })
})
