import type { ReactNode } from "react";
import { redirect } from "next/navigation";
import { AppShell } from "@/components/app-shell";
import { homePathFor } from "@/lib/roles";
import { requireUser } from "@/lib/server-session";

export default async function PortalLayout({ children }: { children: ReactNode }) {
  const user = await requireUser();
  if (user.role !== "SUBCONTRACTOR") redirect(homePathFor(user));
  return (
    <AppShell area="portal" user={user}>
      {children}
    </AppShell>
  );
}
