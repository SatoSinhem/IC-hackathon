import { redirect } from "next/navigation";
import { getSession } from "@/lib/server-session";
import { homePathFor } from "@/lib/roles";

export default async function HomePage() {
  const user = await getSession();
  redirect(user ? homePathFor(user) : "/login");
}
