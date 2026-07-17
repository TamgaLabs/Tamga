import { expect, test } from "@playwright/test";

test.describe("authentication critical journeys", () => {
  test("an unauthenticated visitor is redirected before reaching the new-project form", async ({ page }) => {
    await page.goto("/dashboard/new");

    await expect(page).toHaveURL(/\/login(?:\?.*)?$/);
    await expect(page.getByText("Enter your admin password to continue.", { exact: true })).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
  });

  test("an authenticated admin can navigate from the dashboard to the new-project form", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Password").fill(process.env.E2E_ADMIN_PASSWORD!);
    await page.getByRole("button", { name: "Sign in to console" }).click();

    await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);
    await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();

    await page.getByRole("button", { name: "New Project" }).click();
    await expect(page).toHaveURL(/\/dashboard\/new(?:\?.*)?$/);
    await expect(page.getByLabel("Repository URL")).toBeVisible();
    await expect(page.getByLabel("Project Name")).toBeVisible();
  });
});
