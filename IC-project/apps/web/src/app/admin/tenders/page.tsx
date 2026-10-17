import { PageHeader } from "@/components/page-header";
import { TenderList } from "@/components/tender-views";
import { requireSection } from "@/lib/server-session";
import { listTenders } from "@/lib/tenders";

export const metadata = { title: "Тендеры — IC Tender" };

export default async function Page() {
  await requireSection("/admin/tenders");
  const tenders = await listTenders();
  return (
    <>
      <PageHeader title="Тендеры" description="Все тендеры системы, включая черновики." />
      <TenderList tenders={tenders} basePath="/admin/tenders" />
    </>
  );
}