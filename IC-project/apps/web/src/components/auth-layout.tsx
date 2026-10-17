import type { ReactNode } from "react";
import { Brand } from "@/components/brand";

export function AuthLayout({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return (
    <main className="grid min-h-screen place-items-center px-4 py-10">
      <div className="w-full max-w-md">
        <div className="mb-8 flex justify-center">
          <Brand />
        </div>
        <section aria-labelledby="auth-title" className="rounded-xl border border-border bg-card/90 p-7 shadow-glow backdrop-blur">
          <h1 id="auth-title" className="text-2xl font-semibold tracking-tight">
            {title}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{description}</p>
          <div className="mt-6">{children}</div>
        </section>
        <p className="mt-6 text-center text-xs text-muted-foreground">Закрытая платформа. Учётные записи выдаёт IC Group.</p>
      </div>
    </main>
  );
}
