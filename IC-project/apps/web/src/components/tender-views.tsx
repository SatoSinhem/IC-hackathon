import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  LOT_STATUS_LABELS,
  TENDER_STATUS_LABELS,
  formatDate,
  formatDateTime,
  formatPercent,
  formatTenge,
  type Tender,
} from "@/lib/tenders";

/** The API already removes fields a subcontractor must not see; these views only show what arrived. */
export function TenderList({ tenders, basePath }: { tenders: Tender[]; basePath: string }) {
  if (tenders.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Тендеров пока нет</CardTitle>
          <p className="mt-1 text-sm text-muted-foreground">Когда появятся доступные тендеры, они будут показаны здесь.</p>
        </CardHeader>
      </Card>
    );
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {tenders.map((t) => (
        <li key={t.id}>
          <Link href={`${basePath}/${t.id}`} className="block h-full rounded-lg focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary">
            <Card className="h-full transition-colors hover:border-primary/60">
              <CardHeader>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-xs text-muted-foreground">{t.code}</p>
                    <CardTitle className="mt-1 break-words">{t.title}</CardTitle>
                  </div>
                  <Badge>{TENDER_STATUS_LABELS[t.status] ?? t.status}</Badge>
                </div>
              </CardHeader>
              <CardContent>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                  <dt className="text-muted-foreground">Сумма</dt>
                  <dd className="text-right font-medium text-primary">{formatTenge(t.currentTenderAmount)}</dd>
                  <dt className="text-muted-foreground">Приём откликов до</dt>
                  <dd className="text-right">{formatDateTime(t.deadlineAt)}</dd>
                  <dt className="text-muted-foreground">Лотов</dt>
                  <dd className="text-right">{t.lotsCount}</dd>
                </dl>
              </CardContent>
            </Card>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1 py-2 sm:grid-cols-3 sm:gap-4">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="text-sm sm:col-span-2 whitespace-pre-line break-words">{children}</dd>
    </div>
  );
}

export function TenderDetailView({ tender: t, backHref }: { tender: Tender; backHref: string }) {
  // clientId exists only in the administrator version of the response.
  const isAdminView = t.clientId !== undefined;
  const lots = t.lots ?? [];

  return (
    <div className="space-y-6">
      <Link href={backHref} className="inline-block text-sm text-primary hover:underline">
        ← К списку тендеров
      </Link>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">{t.code}</p>
              <CardTitle className="mt-1 break-words">{t.title}</CardTitle>
            </div>
            <Badge>{TENDER_STATUS_LABELS[t.status] ?? t.status}</Badge>
          </div>
          {t.description ? <p className="mt-2 text-sm text-muted-foreground whitespace-pre-line">{t.description}</p> : null}
        </CardHeader>
        <CardContent>
          <dl className="divide-y divide-border">
            <Row label="Сумма тендера">
              <span className="font-medium text-primary">{formatTenge(t.currentTenderAmount)}</span>
            </Row>
            <Row label="Приём откликов до">{formatDateTime(t.deadlineAt)}</Row>
            <Row label="Начало работ">{formatDate(t.plannedStartAt)}</Row>
            <Row label="Окончание работ">{formatDate(t.plannedEndAt)}</Row>
            <Row label="Необходимая мощность">{t.requiredCapacity ?? "—"}</Row>
            <Row label="Состав работ">{t.worksDescription ?? "—"}</Row>
            <Row label="График обслуживания">{t.serviceSchedule ?? "—"}</Row>
            <Row label="Требования к субподрядчику">{t.subcontractorRequirements ?? "—"}</Row>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Лоты ({lots.length})</CardTitle>
        </CardHeader>
        <CardContent>
          {lots.length === 0 ? (
            <p className="text-sm text-muted-foreground">У тендера нет лотов.</p>
          ) : (
            <ul className="divide-y divide-border">
              {lots.map((l) => (
                <li key={l.id} className="flex flex-wrap items-start justify-between gap-3 py-3">
                  <div className="min-w-0">
                    <p className="font-medium break-words">{l.name}</p>
                    {l.description ? <p className="text-sm text-muted-foreground">{l.description}</p> : null}
                    {isAdminView && l.initialTenderAmount ? (
                      <p className="mt-1 text-xs text-muted-foreground">Начальная сумма: {formatTenge(l.initialTenderAmount)}</p>
                    ) : null}
                  </div>
                  <div className="text-right">
                    <p className="font-medium text-primary">{formatTenge(l.currentTenderAmount)}</p>
                    <Badge>{LOT_STATUS_LABELS[l.status] ?? l.status}</Badge>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {isAdminView ? (
        <Card>
          <CardHeader>
            <CardTitle>Только для администраторов</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">Эти данные не видны субподрядчикам.</p>
          </CardHeader>
          <CardContent>
            <dl className="divide-y divide-border">
              <Row label="Клиент (id)">{t.clientId}</Row>
              <Row label="Выручка клиента">{formatTenge(t.clientRevenue)}</Row>
              <Row label="Начальная сумма">{formatTenge(t.initialTenderAmount)}</Row>
              <Row label="Доля IC: начальная / текущая">
                {formatPercent(t.initialIcSharePercent)} / {formatPercent(t.currentIcSharePercent)}
              </Row>
              <Row label="Видимость">{t.visibilityMode === "INVITED_ONLY" ? "Только приглашённым" : "Всем подходящим"}</Row>
              <Row label="Опубликован">{formatDateTime(t.publishedAt)}</Row>
              <Row label="Создан">{formatDateTime(t.createdAt)}</Row>
              <Row label="Внутренние комментарии">{t.internalComments ?? "—"}</Row>
            </dl>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}