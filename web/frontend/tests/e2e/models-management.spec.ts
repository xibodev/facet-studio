import { expect, test } from "@playwright/test"

test.beforeEach(async ({ page, request }) => {
  await request.post("/__fixture/reset")
  await page.addInitScript(() => {
    localStorage.setItem(
      "facet-studio-tour-state",
      JSON.stringify({ currentStep: "completed", isActive: false }),
    )
  })
})

test("management is responsive and instance-owned", async ({ page }) => {
  await page.goto("/models")
  await expect(page.getByText("Provider instances")).toBeVisible()
  await expect(page.getByText("Mystery protocol")).toBeVisible()
  await expect(page.getByRole("button", { name: "Configure" })).toHaveCount(0)
  await page.getByRole("tab", { name: "Models & Routes" }).click()
  await expect(page.getByText("first/shared").first()).toBeVisible()
  await expect(page.getByText("second/shared").first()).toBeVisible()
  await expect(page.getByText("disabled/shared")).toHaveCount(0)
  await expect(page.getByText("gpt-5.4-mini")).toHaveCount(0)
})

test("catalog sync sends an empty body", async ({ page }) => {
  let body = ""
  page.on("request", (request) => {
    if (request.url().endsWith("/catalog/sync")) body = request.postData() || ""
  })
  await page.goto("/models")
  await page.getByRole("button", { name: "Sync catalog" }).first().click()
  await expect.poll(() => body).toBe("{}")
})

test("route order persists across reload", async ({ page }) => {
  await page.goto("/models")
  await page.getByRole("tab", { name: "Models & Routes" }).click()
  await page.getByRole("button", { name: "Edit" }).click()
  await page.getByRole("button", { name: "Up" }).last().click()
  await page.getByRole("button", { name: "Save route" }).click()
  await page.reload()
  await page.getByRole("tab", { name: "Models & Routes" }).click()
  await page.getByRole("button", { name: "Edit" }).click()
  const ordered = page.locator("[role=dialog] code")
  await expect(ordered.nth(2)).toHaveText("first/shared")
})
