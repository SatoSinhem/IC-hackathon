import type { ReactNode } from "react";
import { Brand } from "@/components/brand";
import { SidebarNav, type NavItem } from "@/components/sidebar-nav";
import { UserMenu } from "@/components/user-menu";
import { ROLE_LABELS, sectionsFor } from "@/lib/roles";
import type { SessionUser } from "@/types/domain";

export function AppShell({ area, user, children }: { area: "admin" | "portal"; user: SessionUser; children: ReactNode }) {
  const items: NavItem[] = sectionsFor(user, area).map(({ href, title, icon }) => ({ href, title, icon }));
  const home = area === "admin" ? "/admin" : "/portal/tenders";

  return (
    <div className="min-h-screen">
      <a
        href="#content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-50 focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-primary-foreground"
      >
        Перейти к содержимому
      </a>
      <header className="sticky top-0 z-30 border-b border-border bg-background/85 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-[1400px] items-center justify-between gap-4 px-4 md:px-6">
          <Brand href={home} />
          <UserMenu email={user.email} roleLabel={ROLE_LABELS[user.role]} />
        </div>
      </header>
      <div className="mx-auto grid max-w-[1400px] gap-6 px-4 py-6 md:grid-cols-[15.5rem_minmax(0,1fr)] md:px-6">
        <aside aria-label={area === "admin" ? "Административная панель" : "Кабинет субподрядчика"}>
          <SidebarNav items={items} label={area === "admin" ? "Разделы администратора" : "Разделы субподрядчика"} />
        </aside>
        <main id="content" tabIndex={-1} className="min-w-0 focus:outline-none">
          {children}
        </main>
      </div>
    </div>
  );
}
