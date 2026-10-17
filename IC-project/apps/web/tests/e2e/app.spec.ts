import { expect, test } from "@playwright/test";
import { PASSWORD, USERS, formAlert, login, logout, navTitles, signIn } from "./helpers";

test.beforeAll(() => {
  if (!PASSWORD) throw new Error("E2E_PASSWORD is not set");
});

test.describe("anonymous visitor", () => {
  for (const path of ["/admin", "/admin/tenders", "/portal/tenders", "/change-password"]) {
    test(`${path} redirects to the login page`, async ({ page }) => {
      await page.goto(path);
      await expect(page).toHaveURL(/\/login\?next=/);
      await expect(page.getByRole("heading", { name: "Вход в IC Tender" })).toBeVisible();
    });
  }

  test("the root page leads to the login page", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/login$/);
  });

  test("the login page offers no registration", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByText(/регистрац|создать аккаунт|sign up/i)).toHaveCount(0);
    await expect(page.getByText("Учётные записи выдаёт IC Group.")).toBeVisible();
  });

  test("unknown pages show a Russian 404", async ({ page }) => {
    const response = await page.goto("/no-such-page");
    expect(response?.status()).toBe(404);
    await expect(page.getByRole("heading", { name: "Страница не найдена" })).toBeVisible();
  });
});

test.describe("login form", () => {
  test("validates fields before calling the server", async ({ page }) => {
    await page.goto("/login");
    let calls = 0;
    await page.route("**/api/v1/auth/login", (route) => {
      calls++;
      return route.continue();
    });
    await page.getByRole("button", { name: "Войти" }).click();
    await expect(page.getByText("Укажите email")).toBeVisible();
    await expect(page.getByText("Укажите пароль")).toBeVisible();
    await page.getByLabel("Email").fill("not-an-email");
    await page.getByLabel("Пароль", { exact: true }).fill("x");
    await page.getByRole("button", { name: "Войти" }).click();
    await expect(page.getByText("Введите корректный email")).toBeVisible();
    expect(calls).toBe(0);
  });

  test("a wrong password shows one generic message and keeps the user on the page", async ({ page }) => {
    await login(page, "ghost-1@e2e.test", "definitely wrong password");
    await expect(formAlert(page)).toHaveText("Неверный email или пароль.");
    await expect(page).toHaveURL(/\/login/);

    await login(page, USERS.admin, "definitely wrong password");
    await expect(formAlert(page)).toHaveText("Неверный email или пароль.");
  });

  test("the password field can be revealed and hidden", async ({ page }) => {
    await page.goto("/login");
    const field = page.getByLabel("Пароль", { exact: true });
    await expect(field).toHaveAttribute("type", "password");
    await page.getByRole("button", { name: "Показать пароль" }).click();
    await expect(field).toHaveAttribute("type", "text");
    await page.getByRole("button", { name: "Скрыть пароль" }).click();
    await expect(field).toHaveAttribute("type", "password");
  });

  test("repeated failures end in a lockout message", async ({ page }) => {
    for (let i = 0; i < 5; i++) {
      await login(page, "lockme@e2e.test", `wrong password number ${i}`);
      await expect(formAlert(page)).toBeVisible();
    }
    await login(page, "lockme@e2e.test", "wrong password again");
    await expect(formAlert(page)).toContainText("Слишком много попыток входа. Повторите через");
  });
});

test.describe("super admin", () => {
  test("signs in, sees every section and signs out", async ({ page }) => {
    await login(page, USERS.admin);
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByRole("heading", { name: "Главная панель", level: 1 })).toBeVisible();
    await expect(page.getByText("Главный администратор").first()).toBeVisible();

    expect(await navTitles(page)).toEqual([
      "Главная панель",
      "Тендеры",
      "Заявки на утверждение",
      "База субподрядчиков",
      "Контракты и объекты",
      "Правки",
      "Операционные данные",
      "Финансы",
      "Пользователи и роли",
      "Настройки",
      "Журнал аудита",
    ]);

    await page.getByRole("link", { name: "Тендеры" }).first().click();
    await expect(page).toHaveURL(/\/admin\/tenders$/);
    await expect(page.getByRole("heading", { name: "Тендеры", level: 1 })).toBeVisible();
    await expect(page.getByText("Раздел ещё не подключён к серверу")).toBeVisible();
    await expect(page.locator("nav#section-nav a[aria-current=page]")).toHaveText("Тендеры");

    await logout(page);
    await page.goto("/admin");
    await expect(page).toHaveURL(/\/login\?next=/);
  });

  test("the back button after logout does not show private pages", async ({ page }) => {
    await signIn(page, USERS.admin);
    await expect(page).toHaveURL(/\/admin$/);
    await page.getByRole("link", { name: "Тендеры" }).first().click();
    await expect(page).toHaveURL(/\/admin\/tenders$/);
    await logout(page);
    for (let i = 0; i < 2; i++) {
      await page.goBack();
      await expect(page.getByText("Раздел ещё не подключён к серверу")).toHaveCount(0);
      await expect(page.getByText("admin@e2e.test")).toHaveCount(0);
    }
  });

  test("every administrative section opens", async ({ page }) => {
    await login(page, USERS.admin);
    await expect(page).toHaveURL(/\/admin$/);
    for (const path of ["tenders", "approvals", "subcontractors", "contracts", "amendments", "operations", "finance", "users", "settings", "audit"]) {
      const response = await page.goto(`/admin/${path}`);
      expect(response?.status(), path).toBe(200);
      await expect(page.locator("main h1"), path).toBeVisible();
      await expect(page.getByText("Раздел ещё не подключён к серверу"), path).toBeVisible();
    }
  });

  test("an administrator cannot enter the subcontractor portal", async ({ page }) => {
    await signIn(page, USERS.admin);
    await page.goto("/portal/tenders");
    await expect(page).toHaveURL(/\/admin$/);
  });

  test("a safe next parameter is honoured after login", async ({ page }) => {
    await login(page, USERS.admin, PASSWORD, "/admin/audit");
    await expect(page).toHaveURL(/\/admin\/audit$/);
  });

  test("a hostile next parameter is ignored", async ({ page }) => {
    await login(page, USERS.admin, PASSWORD, "https://evil.example/phish");
    await expect(page).toHaveURL(/\/admin$/);
    expect(new URL(page.url()).hostname).not.toBe("evil.example");
  });

  test("an already signed-in user is moved away from the login page", async ({ page }) => {
    await login(page, USERS.admin);
    await expect(page).toHaveURL(/\/admin$/);
    await page.goto("/login");
    await expect(page).toHaveURL(/\/admin$/);
  });
});

test.describe("junior admin", () => {
  test("sees only the sections of this role", async ({ page }) => {
    await login(page, USERS.junior);
    await expect(page).toHaveURL(/\/admin$/);
    expect(await navTitles(page)).toEqual(["Главная панель", "Тендеры", "База субподрядчиков", "Контракты и объекты", "Правки", "Операционные данные"]);
    await expect(page.getByText("Дополнительных разрешений нет")).toBeVisible();
  });

  test("is sent back from restricted sections with a notice", async ({ page }) => {
    await signIn(page, USERS.junior);
    for (const path of ["users", "settings", "audit", "approvals", "finance"]) {
      await page.goto(`/admin/${path}`);
      await expect(page, path).toHaveURL(/\/admin\?denied=1$/);
      await expect(page.getByText("У вас нет доступа к запрошенному разделу."), path).toBeVisible();
    }
  });

  test("gets the finance section only with the VIEW_FINANCE grant", async ({ page }) => {
    await login(page, USERS.finance);
    await expect(page).toHaveURL(/\/admin$/);
    expect(await navTitles(page)).toContain("Финансы");
    await expect(page.getByText("Просмотр финансов")).toBeVisible();
    await page.goto("/admin/finance");
    await expect(page.getByRole("heading", { name: "Финансы", level: 1 })).toBeVisible();
    await page.goto("/admin/users");
    await expect(page).toHaveURL(/\/admin\?denied=1$/);
  });
});

test.describe("subcontractor", () => {
  test("lands in the portal with exactly the four sections and no profile item", async ({ page }) => {
    await login(page, USERS.partner);
    await expect(page).toHaveURL(/\/portal\/tenders$/);
    expect(await navTitles(page)).toEqual(["Доступные тендеры", "Мои отклики", "Мои объекты", "Выполненные объекты"]);
    await expect(page.getByText("Мой профиль")).toHaveCount(0);
    await expect(page.getByText("Субподрядчик").first()).toBeVisible();
  });

  test("cannot open any administrative page", async ({ page }) => {
    await login(page, USERS.partner);
    await expect(page).toHaveURL(/\/portal\/tenders$/);
    for (const path of ["/admin", "/admin/tenders", "/admin/subcontractors", "/admin/audit"]) {
      await page.goto(path);
      await expect(page, path).toHaveURL(/\/portal\/tenders$/);
    }
  });

  test("every portal section opens", async ({ page }) => {
    await signIn(page, USERS.partner);
    for (const path of ["tenders", "responses", "objects", "completed"]) {
      const response = await page.goto(`/portal/${path}`);
      expect(response?.status(), path).toBe(200);
      await expect(page.getByText("Раздел ещё не подключён к серверу"), path).toBeVisible();
    }
    await page.goto("/portal");
    await expect(page).toHaveURL(/\/portal\/tenders$/);
  });
});

test.describe("first sign-in with a temporary password", () => {
  test("forces a password change and then opens the application", async ({ page }) => {
    await login(page, USERS.newbie);
    await expect(page).toHaveURL(/\/change-password$/);
    await expect(page.getByText("Для продолжения работы необходимо сменить выданный вам пароль.")).toBeVisible();

    await page.goto("/admin/tenders");
    await expect(page).toHaveURL(/\/change-password$/);

    const next = "A brand new passphrase 2026";
    await page.getByLabel("Текущий пароль").fill("wrong current password!");
    await page.getByLabel("Новый пароль", { exact: true }).fill(next);
    await page.getByLabel("Повторите новый пароль").fill(next);
    await page.getByRole("button", { name: "Сменить пароль" }).click();
    await expect(formAlert(page)).toHaveText("Текущий пароль указан неверно.");

    await page.getByLabel("Текущий пароль").fill(PASSWORD);
    await page.getByLabel("Новый пароль", { exact: true }).fill("short");
    await page.getByLabel("Повторите новый пароль").fill("short");
    await page.getByRole("button", { name: "Сменить пароль" }).click();
    await expect(page.getByText("Не короче 12 символов")).toBeVisible();

    await page.getByLabel("Новый пароль", { exact: true }).fill(next);
    await page.getByLabel("Повторите новый пароль").fill("a different passphrase 2026");
    await page.getByRole("button", { name: "Сменить пароль" }).click();
    await expect(page.getByText("Пароли не совпадают")).toBeVisible();

    await page.getByLabel("Повторите новый пароль").fill(next);
    await page.getByRole("button", { name: "Сменить пароль" }).click();
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByRole("heading", { name: "Главная панель", level: 1 })).toBeVisible();

    await logout(page);
    await login(page, USERS.newbie, PASSWORD);
    await expect(formAlert(page)).toHaveText("Неверный email или пароль.");
    await login(page, USERS.newbie, next);
    await expect(page).toHaveURL(/\/admin$/);
  });
});

test.describe("appearance and accessibility", () => {
  test("uses a black and green palette", async ({ page }) => {
    await page.goto("/login");
    const colors = await page.evaluate(() => {
      const bg = getComputedStyle(document.body).backgroundColor;
      const btn = getComputedStyle(document.querySelector("button[type=submit]")!).backgroundColor;
      return { bg, btn };
    });
    const rgb = (s: string) => (s.match(/\d+(\.\d+)?/g) ?? []).slice(0, 3).map(Number);
    const [br, bg, bb] = rgb(colors.bg);
    expect(Math.max(br, bg, bb), `background ${colors.bg}`).toBeLessThan(25);
    const [r, g, b] = rgb(colors.btn);
    expect(g, `button ${colors.btn}`).toBeGreaterThan(150);
    expect(g).toBeGreaterThan(r + 60);
    expect(g).toBeGreaterThan(b + 60);
  });

  test("form controls have labels and the skip link works", async ({ page }) => {
    await login(page, USERS.admin);
    await expect(page).toHaveURL(/\/admin$/);
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", { name: "Перейти к содержимому" });
    await expect(skip).toBeFocused();
    await skip.press("Enter");
    await expect(page).toHaveURL(/#content$/);
  });

  test("the layout fits a tablet and offers a menu button on a phone", async ({ page }) => {
    await login(page, USERS.admin);
    await page.setViewportSize({ width: 820, height: 1100 });
    await expect(page.locator("nav#section-nav")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

    await page.setViewportSize({ width: 390, height: 800 });
    const toggle = page.getByRole("button", { name: "Разделы" });
    await expect(toggle).toBeVisible();
    await expect(page.locator("nav#section-nav")).toBeHidden();
    await toggle.click();
    await expect(page.locator("nav#section-nav")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });

  test("security headers are present", async ({ page }) => {
    const response = await page.goto("/login");
    const headers = response!.headers();
    expect(headers["x-content-type-options"]).toBe("nosniff");
    expect(headers["x-frame-options"]).toBe("DENY");
    expect(headers["referrer-policy"]).toBe("strict-origin-when-cross-origin");
    expect(headers["x-powered-by"]).toBeUndefined();
  });
});
