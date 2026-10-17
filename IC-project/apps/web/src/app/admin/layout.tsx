import type { ReactNode } from "react";
import { redirect } from "next/navigation";
import { AppShell } from "@/components/app-shell";
import { homePathFor, isAdmin } from "@/lib/roles";
import { requireUser } from "@/lib/server-session";

export default async function AdminLayout({ children }: { children: ReactNode }) {
  const user = await requireUser();
  if (!isAdmin(user.role)) redirect(homePathFor(user));
  return (
    <AppShell area="admin" user={user}>
      {children}
    </AppShell>
  );
}
