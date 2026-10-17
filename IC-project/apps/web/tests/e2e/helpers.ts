import { expect, type Page } from "@playwright/test";

export const PASSWORD = process.env.E2E_PASSWORD ?? "";

export const USERS = {
  admin: "admin@e2e.test",
  junior: "junior@e2e.test",
  finance: "finance@e2e.test",
  partner: "partner@e2e.test",
  newbie: "newbie@e2e.test",
} as const;

export async function login(page: Page, email: string, password = PASSWORD, next?: string) {
  await page.goto(next ? `/login?next=${encodeURIComponent(next)}` : "/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Пароль", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Войти" }).click();
}

/** Signs in and waits until the session cookie is set and the app has left the login page. */
export async function signIn(page: Page, email: string, password = PASSWORD, next?: string) {
  await login(page, email, password, next);
  await page.waitForURL((url) => !url.pathname.startsWith("/login"), { timeout: 10_000 });
}

/** The alert inside the form; Next.js also keeps its own empty role=alert route announcer on every page. */
export function formAlert(page: Page) {
  return page.locator("form [role=alert]");
}

export async function logout(page: Page) {
  await page.getByRole("button", { name: "Выйти" }).click();
  await expect(page).toHaveURL(/\/login$/);
}

export async function navTitles(page: Page): Promise<string[]> {
  const links = page.locator("nav#section-nav a");
  return (await links.allInnerTexts()).map((t) => t.trim());
}
