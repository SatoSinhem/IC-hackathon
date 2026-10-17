import { defineConfig, devices } from "@playwright/test";

// Needs a running stack and the users created by the e2e preparation script, see README.md ("Тесты интерфейса").
export default defineConfig({
  testDir: "tests/e2e",
  // Login limits are shared state, so tests run one after another.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  reporter: [["list"]],
  timeout: 30_000,
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    locale: "ru-RU",
    timezoneId: "Asia/Almaty",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
