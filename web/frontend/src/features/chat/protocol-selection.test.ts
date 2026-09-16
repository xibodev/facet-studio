import { afterEach, describe, expect, it } from "vitest"

import { getChatState, updateChatStore } from "@/store/chat"

import { handlePicoMessage } from "./protocol"

describe("Pico served selection metadata", () => {
  afterEach(() => updateChatStore({ messages: [] }))

  it("prefers actual served target and retains stable identity", () => {
    handlePicoMessage(
      {
        type: "message.create",
        payload: {
          message_id: "m1",
          content: "answer",
          model_name: "legacy-label",
          served_target: "second/shared",
          served_identity:
            "provider_instance:second|instance_target:second/shared",
        },
      },
      "session",
    )
    const message = getChatState().messages[0]
    expect(message.modelName).toBe("second/shared")
    expect(message.servedTarget).toBe("second/shared")
    expect(message.servedIdentity).toContain("provider_instance:second")
  })
})
