"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2 } from "lucide-react";
import { Alert } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { ApiError, apiFetch } from "@/lib/api-client";
import { changePasswordSchema, PASSWORD_MAX, PASSWORD_MIN, type ChangePasswordValues } from "@/lib/schemas";

export function changePasswordErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return "Не удалось сменить пароль. Повторите попытку.";
  switch (error.code) {
    case "wrong_current_password":
      return "Текущий пароль указан неверно.";
    case "weak_password":
      return `Пароль должен содержать от ${PASSWORD_MIN} до ${PASSWORD_MAX} символов, не совпадать с email и с текущим паролем.`;
    case "rate_limited":
      return "Слишком много попыток. Повторите позже.";
    case "unauthenticated":
      return "Сессия истекла. Войдите снова.";
    case "network_error":
      return "Нет связи с сервером. Проверьте подключение и повторите попытку.";
    default:
      return error.status >= 500 ? "Сервер временно недоступен. Повторите позже." : "Не удалось сменить пароль. Повторите попытку.";
  }
}

type FieldName = keyof ChangePasswordValues;

export function ChangePasswordForm({ home, forced }: { home: string; forced: boolean }) {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "", confirmPassword: "" },
  });

  async function onSubmit(values: ChangePasswordValues) {
    setServerError(null);
    try {
      await apiFetch<void>("/api/v1/auth/change-password", {
        method: "POST",
        body: JSON.stringify({ currentPassword: values.currentPassword, newPassword: values.newPassword }),
      });
      router.replace(home);
      router.refresh();
    } catch (error) {
      if (error instanceof ApiError && error.code === "unauthenticated") {
        router.replace("/login");
        return;
      }
      setServerError(changePasswordErrorMessage(error));
    }
  }

  const fields: { name: FieldName; label: string; autoComplete: string; hint?: string }[] = [
    { name: "currentPassword", label: "Текущий пароль", autoComplete: "current-password" },
    { name: "newPassword", label: "Новый пароль", autoComplete: "new-password", hint: `От ${PASSWORD_MIN} до ${PASSWORD_MAX} символов` },
    { name: "confirmPassword", label: "Повторите новый пароль", autoComplete: "new-password" },
  ];

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      {forced ? <Alert variant="warning">Для продолжения работы необходимо сменить выданный вам пароль.</Alert> : null}
      {serverError ? <Alert variant="destructive">{serverError}</Alert> : null}

      {fields.map((f) => {
        const error = errors[f.name];
        const describedBy = [error ? `${f.name}-error` : null, f.hint ? `${f.name}-hint` : null].filter(Boolean).join(" ") || undefined;
        return (
          <div key={f.name} className="space-y-2">
            <Label htmlFor={f.name}>{f.label}</Label>
            <Input id={f.name} type="password" autoComplete={f.autoComplete} aria-invalid={error ? true : undefined} aria-describedby={describedBy} {...register(f.name)} />
            {f.hint ? (
              <p id={`${f.name}-hint`} className="text-xs text-muted-foreground">
                {f.hint}
              </p>
            ) : null}
            {error ? (
              <p id={`${f.name}-error`} className="text-sm text-destructive">
                {error.message}
              </p>
            ) : null}
          </div>
        );
      })}

      <Button type="submit" size="lg" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? <Loader2 aria-hidden className="h-4 w-4 animate-spin" /> : null}
        {isSubmitting ? "Сохранение…" : "Сменить пароль"}
      </Button>
    </form>
  );
}
