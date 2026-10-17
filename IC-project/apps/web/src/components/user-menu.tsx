"use client";

import { useState } from "react";
import Link from "next/link";
import { KeyRound, LogOut } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { apiFetch } from "@/lib/api-client";
import { cn } from "@/lib/utils";

export function UserMenu({ email, roleLabel }: { email: string; roleLabel: string }) {
  const [busy, setBusy] = useState(false);

  async function logout() {
    setBusy(true);
    try {
      await apiFetch<void>("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // The cookie may already be invalid; leaving the page is still correct.
    }
    // A full load drops the client router cache, so the Back button cannot show private pages.
    window.location.replace("/login");
  }

  return (
    <div className="flex items-center gap-2">
      <div className="hidden text-right leading-tight sm:block">
        <div className="max-w-[16rem] truncate text-sm font-medium" title={email}>
          {email}
        </div>
        <Badge variant="muted" className="mt-0.5">
          {roleLabel}
        </Badge>
      </div>
      <Link href="/change-password" className={cn(buttonVariants({ variant: "ghost", size: "sm" }))} aria-label="Сменить пароль">
        <KeyRound aria-hidden className="h-4 w-4" />
        <span className="hidden lg:inline">Пароль</span>
      </Link>
      <Button variant="outline" size="sm" onClick={logout} disabled={busy}>
        <LogOut aria-hidden className="h-4 w-4" />
        Выйти
      </Button>
    </div>
  );
}
