import { PageHeader } from "@/components/page-header";
import { SectionIcon } from "@/components/sidebar-nav";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { requireSection } from "@/lib/server-session";
import { sectionFor } from "@/lib/roles";

/** A section whose API does not exist yet: it says so instead of showing fake data or dead buttons. */
export async function SectionPage({ href }: { href: string }) {
  await requireSection(href);
  const section = sectionFor(href);
  if (!section) return null;

  return (
    <>
      <PageHeader title={section.title} description={section.description} actions={<Badge variant="warning">Не подключено</Badge>} />
      <Card>
        <CardHeader className="flex-row items-center gap-4">
          <span aria-hidden className="grid h-12 w-12 shrink-0 place-items-center rounded-lg border border-primary/30 bg-primary/10 text-primary">
            <SectionIcon name={section.icon} className="h-6 w-6" />
          </span>
          <div>
            <CardTitle>Раздел ещё не подключён к серверу</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">Серверный API для этого раздела пока не реализован, поэтому данных и действий здесь нет.</p>
          </div>
        </CardHeader>
        {section.planned.length > 0 ? (
          <CardContent>
            <h2 className="mb-2 text-sm font-medium">Что здесь появится</h2>
            <ul className="space-y-2 text-sm text-muted-foreground">
              {section.planned.map((item) => (
                <li key={item} className="flex gap-2">
                  <span aria-hidden className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                  {item}
                </li>
              ))}
            </ul>
          </CardContent>
        ) : null}
      </Card>
    </>
  );
}
