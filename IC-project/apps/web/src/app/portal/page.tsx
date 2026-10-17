import { redirect } from "next/navigation";
import { requireSection } from "@/lib/server-session";

export default async function PortalIndexPage() {
  await requireSection("/portal/tenders");
  redirect("/portal/tenders");
}
