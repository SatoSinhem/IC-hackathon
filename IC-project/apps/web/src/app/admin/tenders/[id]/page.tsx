import { PageHeader } from "@/components/page-header";
import { TenderDetailView } from "@/components/tender-views";
import { requireSection } from "@/lib/server-session";
import { getTender } from "@/lib/tenders";

export const metadata = { title: "Тендер — IC Tender" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  await requireSection(`/admin/tenders/${id}`);
  const tender = await getTender(id);
  return (
    <>
      <PageHeader title="Карточка тендера" description={tender.code} />
      <TenderDetailView tender={tender} backHref="/admin/tenders" />
    </>
  );
}