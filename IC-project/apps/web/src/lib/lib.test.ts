import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiFetch } from "@/lib/api-client";
import { formatDate, formatDateTime, formatKzt } from "@/lib/format";
import { changePasswordSchema, loginSchema } from "@/lib/schemas";

const normalize = (s: string) => s.replace(/[\u00a0\u202f]/g, " ");

describe("loginSchema", () => {
  it("accepts a valid pair", () => {
    expect(loginSchema.safeParse({ email: "admin@ic.kz", password: "x" }).success).toBe(true);
  });

  it("explains each problem in Russian", () => {
    const empty = loginSchema.safeParse({ email: "", password: "" });
    expect(empty.success).toBe(false);
    if (!empty.success) {
      const messages = empty.error.issues.map((i) => i.message);
      expect(messages).toContain("Укажите email");
      expect(messages).toContain("Укажите пароль");
    }
    const bad = loginSchema.safeParse({ email: "not-an-email", password: "x" });
    expect(bad.success).toBe(false);
    if (!bad.success) expect(bad.error.issues[0].message).toBe("Введите корректный email");
  });
});

describe("changePasswordSchema", () => {
  const ok = { currentPassword: "old password!", newPassword: "a brand new passphrase", confirmPassword: "a brand new passphrase" };

  it("accepts a good change", () => {
    expect(changePasswordSchema.safeParse(ok).success).toBe(true);
  });

  it("enforces the 12 to 128 character limits of the API", () => {
    expect(changePasswordSchema.safeParse({ ...ok, newPassword: "x".repeat(11), confirmPassword: "x".repeat(11) }).success).toBe(false);
    expect(changePasswordSchema.safeParse({ ...ok, newPassword: "x".repeat(12), confirmPassword: "x".repeat(12) }).success).toBe(true);
    expect(changePasswordSchema.safeParse({ ...ok, newPassword: "x".repeat(129), confirmPassword: "x".repeat(129) }).success).toBe(false);
  });

  it("requires the confirmation to match", () => {
    const r = changePasswordSchema.safeParse({ ...ok, confirmPassword: "something else entirely" });
    expect(r.success).toBe(false);
    if (!r.success) expect(r.error.issues.find((i) => i.path[0] === "confirmPassword")?.message).toBe("Пароли не совпадают");
  });

  it("rejects reusing the current password", () => {
    const r = changePasswordSchema.safeParse({ currentPassword: "same password 123", newPassword: "same password 123", confirmPassword: "same password 123" });
    expect(r.success).toBe(false);
    if (!r.success) expect(r.error.issues.find((i) => i.path[0] === "newPassword")?.message).toMatch(/отличаться/);
  });
});

describe("formatting", () => {
  it("formats tenge with the currency sign and two decimals", () => {
    expect(normalize(formatKzt(1000000))).toBe("1 000 000,00 ₸");
    expect(normalize(formatKzt("850000.5"))).toBe("850 000,50 ₸");
  });

  it("does not print garbage for invalid amounts", () => {
    expect(formatKzt("abc")).toBe("—");
    expect(formatKzt(Number.NaN)).toBe("—");
  });

  it("uses the Asia/Almaty time zone (UTC+5)", () => {
    expect(normalize(formatDateTime("2026-10-17T20:30:00Z"))).toBe("18.10.2026, 01:30");
    expect(formatDate("2026-10-17T20:30:00Z")).toBe("18.10.2026");
    expect(formatDateTime("not a date")).toBe("—");
  });
});

describe("apiFetch", () => {
  afterEach(() => vi.unstubAllGlobals());

  const respond = (status: number, body: unknown, headers: Record<string, string> = {}) =>
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body === null ? null : JSON.stringify(body), { status, headers })));

  it("returns the parsed body on success", async () => {
    respond(200, { user: { id: 1 } });
    await expect(apiFetch("/api/v1/auth/me")).resolves.toEqual({ user: { id: 1 } });
  });

  it("returns undefined for 204", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    await expect(apiFetch("/api/v1/auth/logout", { method: "POST" })).resolves.toBeUndefined();
  });

  it("turns the API error shape into ApiError", async () => {
    respond(401, { error: { code: "invalid_credentials", message: "Неверный email или пароль" } });
    const err = await apiFetch("/x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 401, code: "invalid_credentials", message: "Неверный email или пароль" });
  });

  it("reads Retry-After on rate limiting", async () => {
    respond(429, { error: { code: "rate_limited", message: "x" } }, { "Retry-After": "840" });
    await expect(apiFetch("/x")).rejects.toMatchObject({ status: 429, retryAfterSeconds: 840 });
  });

  it("survives a non-JSON error body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("<html>bad gateway</html>", { status: 502 })));
    await expect(apiFetch("/x")).rejects.toMatchObject({ status: 502, code: "unknown" });
  });

  it("reports a network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("failed")));
    await expect(apiFetch("/x")).rejects.toMatchObject({ status: 0, code: "network_error" });
  });

  it("sends same-origin credentials and JSON by default", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await apiFetch("/api/v1/auth/login", { method: "POST", body: "{}" });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.credentials).toBe("same-origin");
    expect(init.cache).toBe("no-store");
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
  });
});
