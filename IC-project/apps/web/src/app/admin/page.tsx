import Link from "next/link";
import { PageHeader } from "@/components/page-header";
import { SectionIcon } from "@/components/sidebar-nav";
import { Alert, Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PERMISSION_LABELS, ROLE_LABELS, sectionsFor } from "@/lib/roles";
import { getSystemStatus, requireSection } from "@/lib/server-session";

export const metadata = { title: "Главная панель — IC Tender" };

export default async function AdminHomePage({ searchParams }: { searchParams: Promise<{ denied?: string }> }) {
  const user = await requireSection("/admin");
  const { denied } = await searchParams;
  const status = await getSystemStatus();
  const sections = sectionsFor(user, "admin").filter((s) => s.href !== "/admin");

  return (
    <>
      <PageHeader title="Главная панель" description="Разделы, доступные вашей роли." />
      {denied ? (
        <Alert variant="warning" className="mb-4">
          У вас нет доступа к запрошенному разделу.
        </Alert>
      ) : null}

      <div className="mb-6 grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardDescription>Ваша роль</CardDescription>
            <CardTitle>{ROLE_LABELS[user.role]}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            {user.role === "SUPER_ADMIN" ? (
              <Badge>Все разрешения</Badge>
            ) : user.permissions && user.permissions.length > 0 ? (
              user.permissions.map((p) => <Badge key={p}>{PERMISSION_LABELS[p] ?? p}</Badge>)
            ) : (
              <Badge variant="muted">Дополнительных разрешений нет</Badge>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Состояние системы</CardDescription>
            <CardTitle className="flex items-center gap-2">
              <span aria-hidden className={`h-2.5 w-2.5 rounded-full ${status.ok ? "bg-primary" : "bg-destructive"}`} />
              {status.ok ? "Работает" : "Проблема"}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">{status.detail}</CardContent>
        </Card>
      </div>

      <h2 className="mb-3 text-sm font-medium text-muted-foreground">Разделы</h2>
      <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {sections.map((s) => (
          <li key={s.href}>
            <Link href={s.href} className="group block h-full rounded-lg border border-border bg-card p-5 transition-colors hover:border-primary/50 hover:bg-accent/40">
              <span aria-hidden className="mb-3 grid h-10 w-10 place-items-center rounded-md bg-primary/10 text-primary">
                <SectionIcon name={s.icon} className="h-5 w-5" />
              </span>
              <span className="block font-medium group-hover:text-accent-foreground">{s.title}</span>
              <span className="mt-1 block text-sm text-muted-foreground">{s.description}</span>
            </Link>
          </li>
        ))}
      </ul>
    </>
  );
}
