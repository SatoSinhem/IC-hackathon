export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly retryAfterSeconds?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function errorFields(data: unknown): { code?: string; message?: string } {
  if (typeof data !== "object" || data === null || !("error" in data)) return {};
  const e = (data as { error: unknown }).error;
  if (typeof e !== "object" || e === null) return {};
  const { code, message } = e as { code?: unknown; message?: unknown };
  return { code: typeof code === "string" ? code : undefined, message: typeof message === "string" ? message : undefined };
}

function parseRetryAfter(value: string | null): number | undefined {
  const n = value === null ? NaN : Number(value);
  return Number.isFinite(n) && n > 0 ? Math.ceil(n) : undefined;
}

/** Same-origin call: Next.js proxies /api/* to the Go API, so the session cookie never leaves this origin. */
export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      ...init,
      credentials: "same-origin",
      cache: "no-store",
      headers: { "Content-Type": "application/json", ...init.headers },
    });
  } catch {
    throw new ApiError(0, "network_error", "Нет связи с сервером");
  }
  if (res.status === 204) return undefined as T;
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    data = null;
  }
  if (!res.ok) {
    const { code, message } = errorFields(data);
    throw new ApiError(res.status, code ?? "unknown", message ?? `Ошибка ${res.status}`, parseRetryAfter(res.headers.get("Retry-After")));
  }
  return data as T;
}
