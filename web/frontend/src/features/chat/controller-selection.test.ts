import { describe, expect, it } from "vitest"

import { buildChatMessagePayload } from "./controller"

describe("buildChatMessagePayload", () => {
  it("preserves legacy payload when selection is absent", () => {
    expect(buildChatMessagePayload({ content: " hello " })).toEqual({
      content: "hello",
      media: [],
    })
  })

  it("sends exact selection for normal and context messages", () => {
    expect(
      buildChatMessagePayload({ content: "hello", selection: " first/model " }),
    ).toMatchObject({ selection: "first/model" })
    expect(
      buildChatMessagePayload({ content: "/context", selection: "primary" }),
    ).toEqual({ content: "/context", media: [], selection: "primary" })
  })
})
