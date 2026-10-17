import { cache } from "react";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { canAccess, homePathFor, sectionFor } from "@/lib/roles";
import type { SessionUser } from "@/types/domain";

const API_URL = (process.env.API_INTERNAL_URL ?? "http://localhost:8080").replace(/\/+$/, "");
const SESSION_COOKIES = ["__Host-ic_session", "ic_session"];

export class ApiUnavailableError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ApiUnavailableError";
  }
}

/** Resolves the current user by asking the API; the cookie is only a pointer, never trusted on its own. */
export const getSession = cache(async (): Promise<SessionUser | null> => {
  const jar = await cookies();
  const cookie = SESSION_COOKIES.map((name) => jar.get(name)).find((c) => c !== undefined);
  if (!cookie) return null;

  let res: Response;
  try {
    res = await fetch(`${API_URL}/api/v1/auth/me`, {
      headers: { cookie: `${cookie.name}=${cookie.value}` },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    throw new ApiUnavailableError("Не удалось связаться с API");
  }
  if (res.status === 401) return null;
  if (!res.ok) throw new ApiUnavailableError(`API ответил статусом ${res.status}`);
  const body = (await res.json()) as { user: SessionUser };
  return body.user;
});

export async function requireUser(options: { allowPasswordChange?: boolean } = {}): Promise<SessionUser> {
  const user = await getSession();
  if (!user) redirect("/login");
  if (user.mustChangePassword && !options.allowPasswordChange) redirect("/change-password");
  return user;
}

/** Every page calls this itself: layouts are not re-rendered on client-side navigation, so they cannot be the only check. */
export async function requireSection(pathname: string): Promise<SessionUser> {
  const user = await requireUser();
  const section = sectionFor(pathname);
  if (!section || !canAccess(user, section)) {
    const home = homePathFor(user);
    redirect(home === "/admin" ? "/admin?denied=1" : home);
  }
  return user;
}

export type SystemStatus = { ok: boolean; detail: string };

export async function getSystemStatus(): Promise<SystemStatus> {
  try {
    const res = await fetch(`${API_URL}/api/v1/ready`, { cache: "no-store", signal: AbortSignal.timeout(3000) });
    return res.ok ? { ok: true, detail: "API и база данных доступны" } : { ok: false, detail: "База данных недоступна" };
  } catch {
    return { ok: false, detail: "API не отвечает" };
  }
}
