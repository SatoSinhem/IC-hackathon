import type { SessionUser, UserRole } from "@/types/domain";

export const ROLE_LABELS: Record<UserRole, string> = {
  SUPER_ADMIN: "Главный администратор",
  JUNIOR_ADMIN: "Младший администратор",
  SUBCONTRACTOR: "Субподрядчик",
};

/** Permission names must match the values stored in user_permissions. */
export const PERMISSIONS = {
  VIEW_FINANCE: "VIEW_FINANCE",
  AWARD_WINNER: "AWARD_WINNER",
} as const;

export const PERMISSION_LABELS: Record<string, string> = {
  VIEW_FINANCE: "Просмотр финансов",
  AWARD_WINNER: "Назначение победителя",
};

export type IconName =
  | "layout-dashboard"
  | "file-text"
  | "check-square"
  | "users"
  | "building"
  | "file-pen"
  | "calendar-check"
  | "wallet"
  | "shield"
  | "settings"
  | "history"
  | "inbox"
  | "send"
  | "map-pin"
  | "badge-check";

export type Section = {
  href: string;
  title: string;
  description: string;
  icon: IconName;
  area: "admin" | "portal";
  roles: UserRole[];
  /** Junior administrators need this grant; SUPER_ADMIN holds every permission. */
  permission?: string;
  planned: string[];
};

const ADMINS: UserRole[] = ["SUPER_ADMIN", "JUNIOR_ADMIN"];

export const SECTIONS: Section[] = [
  {
    href: "/admin",
    title: "Главная панель",
    description: "Сводка и состояние системы.",
    icon: "layout-dashboard",
    area: "admin",
    roles: ADMINS,
    planned: [],
  },
  {
    href: "/admin/tenders",
    title: "Тендеры",
    description: "Создание, утверждение, публикация и ручной выбор победителя.",
    icon: "file-text",
    area: "admin",
    roles: ADMINS,
    planned: [
      "Список тендеров с фильтрами и статусами",
      "Анкета тендера и расчёт суммы по доле IC",
      "Отправка на утверждение, публикация, закрытие приёма откликов",
      "Просмотр откликов и ручное назначение победителя",
    ],
  },
  {
    href: "/admin/approvals",
    title: "Заявки на утверждение",
    description: "Тендеры и правки, ожидающие решения главного администратора.",
    icon: "check-square",
    area: "admin",
    roles: ["SUPER_ADMIN"],
    planned: ["Очередь тендеров на утверждение", "Утверждение, возврат на доработку или отклонение", "Утверждение правок с финансовым эффектом"],
  },
  {
    href: "/admin/subcontractors",
    title: "База субподрядчиков",
    description: "Закрытый внутренний справочник IC Group.",
    icon: "users",
    area: "admin",
    roles: ADMINS,
    planned: [
      "Карточки партнёров, регионы и специализации",
      "Мощность, текущая загрузка и свободная мощность",
      "Рейтинг, отзывы и история сотрудничества (только для администраторов)",
      "Готовность обучать операторов",
    ],
  },
  {
    href: "/admin/contracts",
    title: "Контракты и объекты",
    description: "Действующие контракты, объекты и подтверждение завершения.",
    icon: "building",
    area: "admin",
    roles: ADMINS,
    planned: ["Карточки контрактов и связанных объектов", "Статусы: черновик, действует, ожидает закрытия, завершён", "Явное подтверждение завершения администратором"],
  },
  {
    href: "/admin/amendments",
    title: "Правки",
    description: "Дополнительные услуги, штрафные корректировки и изменения условий.",
    icon: "file-pen",
    area: "admin",
    roles: ADMINS,
    planned: ["Кнопка «+ Добавить» дополнительную услугу с подтверждением согласования", "Проекты штрафных корректировок с основанием и доказательствами", "История изменений и утверждение"],
  },
  {
    href: "/admin/operations",
    title: "Операционные данные",
    description: "Импорт данных, смены, замечания и закрытие месяца.",
    icon: "calendar-check",
    area: "admin",
    roles: ADMINS,
    planned: ["Импорт файлов пакета без потери исходных значений", "Плановые и фактические смены, конфликты расписания", "Замечания клиентов и акты субподрядчиков", "Закрытие месяца"],
  },
  {
    href: "/admin/finance",
    title: "Финансы",
    description: "Внутренняя маржа IC и финансовые показатели.",
    icon: "wallet",
    area: "admin",
    roles: ADMINS,
    permission: PERMISSIONS.VIEW_FINANCE,
    planned: ["Доля IC и опубликованные суммы тендеров", "История изменений цены при приближении дедлайна", "Расчёты закрытия месяца"],
  },
  {
    href: "/admin/users",
    title: "Пользователи и роли",
    description: "Учётные записи, роли и дополнительные разрешения.",
    icon: "shield",
    area: "admin",
    roles: ["SUPER_ADMIN"],
    planned: ["Создание и приостановка учётных записей", "Назначение ролей и разрешений", "Принудительная смена пароля"],
  },
  {
    href: "/admin/settings",
    title: "Настройки",
    description: "Ценовая политика и системные параметры.",
    icon: "settings",
    area: "admin",
    roles: ["SUPER_ADMIN"],
    planned: ["Базовая и минимальная доля IC", "Ступени изменения доли при приближении дедлайна"],
  },
  {
    href: "/admin/audit",
    title: "Журнал аудита",
    description: "Журнал критических действий в системе.",
    icon: "history",
    area: "admin",
    roles: ["SUPER_ADMIN"],
    planned: ["Поиск и фильтрация записей", "Значения до и после изменения", "Автор, время и основание действия"],
  },
  {
    href: "/portal/tenders",
    title: "Доступные тендеры",
    description: "Опубликованные тендеры и отклик по установленной IC цене.",
    icon: "inbox",
    area: "portal",
    roles: ["SUBCONTRACTOR"],
    planned: ["Список опубликованных и доступных вам тендеров", "Условия тендера и сумма, установленная IC Group", "Подтверждение готовности, мощности и даты начала"],
  },
  {
    href: "/portal/responses",
    title: "Мои отклики",
    description: "Статусы отправленных откликов.",
    icon: "send",
    area: "portal",
    roles: ["SUBCONTRACTOR"],
    planned: ["Статусы откликов", "Подтверждение обновлённой суммы при изменении условий", "Отзыв отклика"],
  },
  {
    href: "/portal/objects",
    title: "Мои объекты",
    description: "Объекты и контракты, назначенные вашей компании.",
    icon: "map-pin",
    area: "portal",
    roles: ["SUBCONTRACTOR"],
    planned: ["Действующие контракты и объекты", "Статус «Ожидает закрытия» после окончания срока", "Операционные сведения по дополнительным услугам"],
  },
  {
    href: "/portal/completed",
    title: "Выполненные объекты",
    description: "Контракты, завершение которых подтвердил администратор.",
    icon: "badge-check",
    area: "portal",
    roles: ["SUBCONTRACTOR"],
    planned: ["Название объекта, город, номер контракта, даты", "Статус «Завершён» и краткий объём работ", "Итоговые документы, если их выдача разрешена"],
  },
];

export function isAdmin(role: UserRole): boolean {
  return role === "SUPER_ADMIN" || role === "JUNIOR_ADMIN";
}

export function canAccess(user: Pick<SessionUser, "role" | "permissions">, section: Section): boolean {
  if (!section.roles.includes(user.role)) return false;
  if (!section.permission || user.role === "SUPER_ADMIN") return true;
  return user.permissions?.includes(section.permission) ?? false;
}

export function sectionsFor(user: Pick<SessionUser, "role" | "permissions">, area: "admin" | "portal"): Section[] {
  return SECTIONS.filter((s) => s.area === area && canAccess(user, s));
}

export function homePathFor(user: Pick<SessionUser, "role" | "mustChangePassword">): string {
  if (user.mustChangePassword) return "/change-password";
  return isAdmin(user.role) ? "/admin" : "/portal/tenders";
}

/** Finds the section that owns a path; the longest matching href wins. */
export function sectionFor(pathname: string): Section | undefined {
  return [...SECTIONS]
    .sort((a, b) => b.href.length - a.href.length)
    .find((s) => pathname === s.href || pathname.startsWith(s.href + "/"));
}

/** Returns a same-site path the user may open, or null; blocks open redirects such as //evil.example. */
export function safeNext(next: string | null | undefined, user: Pick<SessionUser, "role" | "permissions">): string | null {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.includes("\\")) return null;
  let pathname: string;
  try {
    pathname = new URL(next, "http://localhost").pathname;
  } catch {
    return null;
  }
  const section = sectionFor(pathname);
  return section && canAccess(user, section) ? pathname : null;
}
