import { PageHeader } from "@/components/page-header";
import { TenderList } from "@/components/tender-views";
import { requireSection } from "@/lib/server-session";
import { listTenders } from "@/lib/tenders";

export const metadata = { title: "Доступные тендеры — IC Tender" };

export default async function Page() {
  await requireSection("/portal/tenders");
  const tenders = await listTenders();
  return (
    <>
      <PageHeader title="Доступные тендеры" description="Опубликованные тендеры, доступные вашей компании." />
      <TenderList tenders={tenders} basePath="/portal/tenders" />
    </>
  );
}