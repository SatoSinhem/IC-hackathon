import { cookies } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { ApiUnavailableError } from "@/lib/server-session";

const API_URL = (process.env.API_INTERNAL_URL ?? "http://localhost:8080").replace(/\/+$/, "");
const SESSION_COOKIES = ["__Host-ic_session", "ic_session"];

export type Lot = {
  id: number;
  name: string;
  description: string | null;
  status: string;
  currentTenderAmount: string;
  /** Present for administrators only. */
  initialTenderAmount?: string;
  awardedSubcontractorId?: number | null;
  priceVersion?: number;
};

export type Tender = {
  id: number;
  code: string;
  title: string;
  description: string | null;
  status: string;
  publishedAt: string | null;
  deadlineAt: string;
  plannedStartAt: string | null;
  plannedEndAt: string | null;
  currentTenderAmount: string;
  worksDescription: string | null;
  serviceSchedule: string | null;
  requiredCapacity: number | null;
  subcontractorRequirements: string | null;
  lotsCount: number;
  lots?: Lot[];
  /** Everything below is sent by the API to administrators only. */
  visibilityMode?: string;
  clientId?: number;
  createdBy?: number;
  approvedBy?: number | null;
  clientRevenue?: string | null;
  initialIcSharePercent?: string;
  currentIcSharePercent?: string;
  initialTenderAmount?: string;
  internalComments?: string | null;
  awardedAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

async function apiGet<T>(path: string): Promise<T> {
  const jar = await cookies();
  const cookie = SESSION_COOKIES.map((name) => jar.get(name)).find((c) => c !== undefined);
  if (!cookie) redirect("/login");

  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      headers: { cookie: `${cookie.name}=${cookie.value}` },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    throw new ApiUnavailableError("Не удалось связаться с API");
  }
  if (res.status === 401) redirect("/login");
  if (res.status === 403) redirect("/");
  if (res.status === 404) notFound();
  if (!res.ok) throw new ApiUnavailableError(`API ответил статусом ${res.status}`);
  return (await res.json()) as T;
}

export async function listTenders(): Promise<Tender[]> {
  const body = await apiGet<{ tenders: Tender[] }>("/api/v1/tenders");
  return body.tenders;
}

export async function getTender(id: string): Promise<Tender> {
  if (!/^\d{1,15}$/.test(id)) notFound();
  const body = await apiGet<{ tender: Tender }>(`/api/v1/tenders/${id}`);
  return body.tender;
}

export const TENDER_STATUS_LABELS: Record<string, string> = {
  DRAFT: "Черновик",
  PENDING_APPROVAL: "На утверждении",
  CHANGES_REQUESTED: "Нужна доработка",
  APPROVED: "Утверждён",
  PUBLISHED: "Опубликован",
  CLOSED: "Приём откликов закрыт",
  AWARDED: "Победитель назначен",
  IN_PROGRESS: "В работе",
  COMPLETED: "Завершён",
  CANCELLED: "Отменён",
};

export const LOT_STATUS_LABELS: Record<string, string> = {
  OPEN: "Открыт",
  CLOSED: "Закрыт",
  AWARDED: "Назначен",
  CANCELLED: "Отменён",
};

const money = new Intl.NumberFormat("ru-RU", { style: "currency", currency: "KZT", maximumFractionDigits: 2 });
const dateTime = new Intl.DateTimeFormat("ru-RU", { timeZone: "Asia/Almaty", dateStyle: "medium", timeStyle: "short" });
const dateOnly = new Intl.DateTimeFormat("ru-RU", { timeZone: "Asia/Almaty", dateStyle: "medium" });

/** Amounts arrive from the API as decimal strings such as "1500000.00". */
export function formatTenge(value: string | null | undefined): string {
  if (value == null) return "—";
  const n = Number(value);
  return Number.isFinite(n) ? money.format(n) : "—";
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "—" : dateTime.format(d);
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "—" : dateOnly.format(d);
}

export function formatPercent(value: string | null | undefined): string {
  if (value == null) return "—";
  const n = Number(value);
  return Number.isFinite(n) ? `${n.toLocaleString("ru-RU", { maximumFractionDigits: 2 })} %` : "—";
}