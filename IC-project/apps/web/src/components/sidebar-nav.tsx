"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BadgeCheck,
  Building2,
  CalendarCheck,
  CheckSquare,
  FilePen,
  FileText,
  History,
  Inbox,
  LayoutDashboard,
  Menu,
  MapPin,
  Send,
  Settings,
  Shield,
  Users,
  Wallet,
  X,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { IconName } from "@/lib/roles";

const ICONS: Record<IconName, LucideIcon> = {
  "layout-dashboard": LayoutDashboard,
  "file-text": FileText,
  "check-square": CheckSquare,
  users: Users,
  building: Building2,
  "file-pen": FilePen,
  "calendar-check": CalendarCheck,
  wallet: Wallet,
  shield: Shield,
  settings: Settings,
  history: History,
  inbox: Inbox,
  send: Send,
  "map-pin": MapPin,
  "badge-check": BadgeCheck,
};

export function SectionIcon({ name, className }: { name: IconName; className?: string }) {
  const Icon = ICONS[name];
  return <Icon aria-hidden className={className} />;
}

export type NavItem = { href: string; title: string; icon: IconName };

export function SidebarNav({ items, label }: { items: NavItem[]; label: string }) {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  // The dashboard href is a prefix of every admin path, so it matches exactly only.
  const isActive = (href: string) => (href === "/admin" ? pathname === href : pathname === href || pathname.startsWith(href + "/"));

  return (
    <div>
      <button
        type="button"
        className="mb-3 inline-flex h-10 items-center gap-2 rounded-md border border-border bg-card px-3 text-sm md:hidden"
        aria-expanded={open}
        aria-controls="section-nav"
        onClick={() => setOpen((v) => !v)}
      >
        {open ? <X aria-hidden className="h-4 w-4" /> : <Menu aria-hidden className="h-4 w-4" />}
        Разделы
      </button>
      <nav id="section-nav" aria-label={label} className={cn("flex-col gap-1", open ? "flex" : "hidden", "md:flex")}>
        {items.map((item) => {
          const active = isActive(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              onClick={() => setOpen(false)}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2.5 text-sm transition-colors",
                active ? "bg-accent text-accent-foreground shadow-[inset_2px_0_0_hsl(var(--primary))]" : "text-muted-foreground hover:bg-secondary hover:text-foreground",
              )}
            >
              <SectionIcon name={item.icon} className="h-4 w-4 shrink-0" />
              <span>{item.title}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
