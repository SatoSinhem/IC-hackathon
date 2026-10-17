import { redirect } from "next/navigation";
import { AuthLayout } from "@/components/auth-layout";
import { getSession } from "@/lib/server-session";
import { homePathFor } from "@/lib/roles";
import { LoginForm } from "./login-form";

export const metadata = { title: "Вход — IC Tender" };

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string | string[] }> }) {
  const user = await getSession();
  if (user) redirect(homePathFor(user));

  const { next } = await searchParams;
  return (
    <AuthLayout title="Вход в IC Tender" description="Введите email и пароль, выданные IC Group.">
      <LoginForm next={typeof next === "string" ? next : undefined} />
    </AuthLayout>
  );
}
