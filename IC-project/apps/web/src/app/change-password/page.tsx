import { AuthLayout } from "@/components/auth-layout";
import { requireUser } from "@/lib/server-session";
import { isAdmin } from "@/lib/roles";
import { ChangePasswordForm } from "./change-password-form";

export const metadata = { title: "Смена пароля — IC Tender" };

export default async function ChangePasswordPage() {
  const user = await requireUser({ allowPasswordChange: true });
  const home = isAdmin(user.role) ? "/admin" : "/portal/tenders";
  return (
    <AuthLayout title="Смена пароля" description={user.email}>
      <ChangePasswordForm home={home} forced={user.mustChangePassword} />
    </AuthLayout>
  );
}
