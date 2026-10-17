const kzt = new Intl.NumberFormat("ru-KZ", { style: "currency", currency: "KZT", minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Amounts arrive as decimal strings; doubles are exact for tenge values below 2^53 / 100. */
export function formatKzt(amount: number | string): string {
  const value = typeof amount === "string" ? Number(amount) : amount;
  return Number.isFinite(value) ? kzt.format(value) : "—";
}

const dateTime = new Intl.DateTimeFormat("ru-RU", { timeZone: "Asia/Almaty", day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
const dateOnly = new Intl.DateTimeFormat("ru-RU", { timeZone: "Asia/Almaty", day: "2-digit", month: "2-digit", year: "numeric" });

export function formatDateTime(value: string | Date): string {
  const d = typeof value === "string" ? new Date(value) : value;
  return Number.isNaN(d.getTime()) ? "—" : dateTime.format(d);
}

export function formatDate(value: string | Date): string {
  const d = typeof value === "string" ? new Date(value) : value;
  return Number.isNaN(d.getTime()) ? "—" : dateOnly.format(d);
}
