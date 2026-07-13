import { request, type FullConfig } from "@playwright/test";

type SetupStatus = { setup: boolean };

function requiredEnvironment(name: "E2E_BASE_URL" | "E2E_ADMIN_PASSWORD" | "E2E_OWNED_STACK") {
  const value = process.env[name];
  if (!value || (name === "E2E_OWNED_STACK" && value !== "1")) {
    throw new Error(
      "Browser E2E requires E2E_BASE_URL, E2E_ADMIN_PASSWORD, and E2E_OWNED_STACK=1 for a CI-owned disposable Tamga stack."
    );
  }
  return value;
}

/**
 * Bootstraps only the disposable stack acknowledged by E2E_OWNED_STACK=1.
 * A fresh stack receives its configured test password; an existing stack must
 * already accept that password. No projects are created, so this suite has no
 * project/container cleanup that could affect another test or operator data.
 */
export default async function globalSetup(_config: FullConfig) {
  const baseURL = requiredEnvironment("E2E_BASE_URL");
  const password = requiredEnvironment("E2E_ADMIN_PASSWORD");
  requiredEnvironment("E2E_OWNED_STACK");

  const api = await request.newContext({ baseURL, ignoreHTTPSErrors: true });
  try {
    const status = await api.get("/api/auth/status");
    if (!status.ok()) {
      throw new Error(`E2E target did not return auth status (HTTP ${status.status()}).`);
    }

    const { setup } = (await status.json()) as SetupStatus;
    if (!setup) {
      const bootstrap = await api.post("/api/auth/setup", { data: { password } });
      if (!bootstrap.ok()) {
        throw new Error(`E2E auth bootstrap failed (HTTP ${bootstrap.status()}).`);
      }
    }

    const login = await api.post("/api/auth/login", { data: { password } });
    if (!login.ok()) {
      throw new Error(
        `E2E target rejected E2E_ADMIN_PASSWORD (HTTP ${login.status()}). Recreate the disposable fixture or provide its seeded password.`
      );
    }
  } finally {
    await api.dispose();
  }
}
