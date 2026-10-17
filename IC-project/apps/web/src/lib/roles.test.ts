import { describe, expect, it } from "vitest";
import { PERMISSIONS, SECTIONS, canAccess, homePathFor, safeNext, sectionFor, sectionsFor } from "@/lib/roles";

const superAdmin = { role: "SUPER_ADMIN" as const, permissions: [] as string[] };
const junior = { role: "JUNIOR_ADMIN" as const, permissions: [] as string[] };
const financeJunior = { role: "JUNIOR_ADMIN" as const, permissions: [PERMISSIONS.VIEW_FINANCE] };
const partner = { role: "SUBCONTRACTOR" as const, permissions: [] as string[] };

const titles = (user: { role: "SUPER_ADMIN" | "JUNIOR_ADMIN" | "SUBCONTRACTOR"; permissions: string[] }, area: "admin" | "portal") => sectionsFor(user, area).map((s) => s.title);

describe("navigation by role", () => {
  it("shows a super admin every administrative section", () => {
    expect(titles(superAdmin, "admin")).toHaveLength(11);
  });

  it("hides approvals, users, settings, audit and finance from a junior admin", () => {
    const visible = titles(junior, "admin");
    for (const hidden of ["Заявки на утверждение", "Пользователи и роли", "Настройки", "Журнал аудита", "Финансы"]) {
      expect(visible).not.toContain(hidden);
    }
    expect(visible).toContain("Тендеры");
    expect(visible).toContain("Правки");
  });

  it("shows finance to a junior admin only with the VIEW_FINANCE grant", () => {
    expect(titles(financeJunior, "admin")).toContain("Финансы");
    expect(titles({ role: "JUNIOR_ADMIN", permissions: ["AWARD_WINNER"] }, "admin")).not.toContain("Финансы");
  });

  it("gives a subcontractor exactly the four portal sections and no admin ones", () => {
    expect(titles(partner, "portal")).toEqual(["Доступные тендеры", "Мои отклики", "Мои объекты", "Выполненные объекты"]);
    expect(sectionsFor(partner, "admin")).toEqual([]);
    expect(SECTIONS.some((s) => /профиль/i.test(s.title))).toBe(false);
  });

  it("keeps administrators out of the subcontractor portal", () => {
    expect(sectionsFor(superAdmin, "portal")).toEqual([]);
    expect(sectionsFor(junior, "portal")).toEqual([]);
  });

  it("never lets a subcontractor reach an administrative section", () => {
    for (const s of SECTIONS.filter((x) => x.area === "admin")) expect(canAccess(partner, s)).toBe(false);
  });
});

describe("homePathFor", () => {
  it("sends users to their own area", () => {
    expect(homePathFor({ role: "SUPER_ADMIN", mustChangePassword: false })).toBe("/admin");
    expect(homePathFor({ role: "JUNIOR_ADMIN", mustChangePassword: false })).toBe("/admin");
    expect(homePathFor({ role: "SUBCONTRACTOR", mustChangePassword: false })).toBe("/portal/tenders");
  });

  it("sends everyone with a temporary password to the password change", () => {
    expect(homePathFor({ role: "SUPER_ADMIN", mustChangePassword: true })).toBe("/change-password");
    expect(homePathFor({ role: "SUBCONTRACTOR", mustChangePassword: true })).toBe("/change-password");
  });
});

describe("sectionFor", () => {
  it("prefers the most specific section", () => {
    expect(sectionFor("/admin")?.title).toBe("Главная панель");
    expect(sectionFor("/admin/tenders")?.title).toBe("Тендеры");
    expect(sectionFor("/admin/tenders/42/edit")?.title).toBe("Тендеры");
  });

  it("does not match a path that merely shares a prefix", () => {
    expect(sectionFor("/admin-evil")).toBeUndefined();
    expect(sectionFor("/administrator")).toBeUndefined();
    expect(sectionFor("/portal")).toBeUndefined();
  });
});

describe("safeNext", () => {
  it("accepts a section the user may open", () => {
    expect(safeNext("/admin/tenders", superAdmin)).toBe("/admin/tenders");
    expect(safeNext("/portal/objects", partner)).toBe("/portal/objects");
  });

  it("drops the query string and fragment", () => {
    expect(safeNext("/admin/tenders?x=1#top", superAdmin)).toBe("/admin/tenders");
  });

  it("rejects redirects to other sites", () => {
    for (const evil of ["https://evil.example", "//evil.example", "/\\evil.example", "http://localhost:3000/admin", "javascript:alert(1)", "evil.example"]) {
      expect(safeNext(evil, superAdmin), evil).toBeNull();
    }
  });

  it("rejects sections the role cannot open", () => {
    expect(safeNext("/admin/users", junior)).toBeNull();
    expect(safeNext("/admin", partner)).toBeNull();
    expect(safeNext("/portal/tenders", superAdmin)).toBeNull();
  });

  it("rejects empty and unknown targets", () => {
    expect(safeNext(undefined, superAdmin)).toBeNull();
    expect(safeNext("", superAdmin)).toBeNull();
    expect(safeNext("/nowhere", superAdmin)).toBeNull();
  });
});
