import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import "@/i18n"

import { ModelSelector } from "./model-selector"
import { chatSelectionValues } from "./model-selector.utils"

describe("ModelSelector", () => {
  it("derives configured default, routes, and distinct exact targets", () => {
    const targets = [
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
    ]
    const routes = [{ name: "primary", targets: ["first/shared"] }]
    render(
      <ModelSelector
        selection=""
        routes={routes}
        targets={targets}
        onValueChange={vi.fn()}
      />,
    )
    expect(screen.getByRole("combobox")).toHaveTextContent(
      "Use configured default",
    )
    expect(chatSelectionValues(targets, routes)).toEqual({
      routes: ["primary"],
      targets: ["first/shared", "second/shared"],
    })
    expect(JSON.stringify(chatSelectionValues(targets, routes))).not.toContain(
      "gpt-5.4-mini",
    )
  })
})
