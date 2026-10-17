"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { Alert } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { ApiError, apiFetch } from "@/lib/api-client";
import { homePathFor, safeNext } from "@/lib/roles";
import { loginSchema, type LoginValues } from "@/lib/schemas";
import type { SessionUser } from "@/types/domain";

export function loginErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return "Не удалось войти. Повторите попытку.";
  switch (error.code) {
    case "invalid_credentials":
      return "Неверный email или пароль.";
    case "rate_limited": {
      const minutes = error.retryAfterSeconds ? Math.max(1, Math.ceil(error.retryAfterSeconds / 60)) : null;
      return minutes ? `Слишком много попыток входа. Повторите через ${minutes} мин.` : "Слишком много попыток входа. Повторите позже.";
    }
    case "forbidden_origin":
      return "Запрос отклонён. Откройте сайт по адресу, который выдал администратор.";
    case "network_error":
      return "Нет связи с сервером. Проверьте подключение и повторите попытку.";
    default:
      return error.status >= 500 ? "Сервер временно недоступен. Повторите позже." : "Не удалось войти. Повторите попытку.";
  }
}

export function LoginForm({ next }: { next?: string }) {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { email: "", password: "" } });

  async function onSubmit(values: LoginValues) {
    setServerError(null);
    try {
      const { user } = await apiFetch<{ user: SessionUser }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: values.email.trim().toLowerCase(), password: values.password }),
      });
      router.replace(user.mustChangePassword ? "/change-password" : (safeNext(next, user) ?? homePathFor(user)));
      router.refresh();
    } catch (error) {
      setServerError(loginErrorMessage(error));
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      {serverError ? <Alert variant="destructive">{serverError}</Alert> : null}

      <div className="space-y-2">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          autoFocus
          aria-invalid={errors.email ? true : undefined}
          aria-describedby={errors.email ? "email-error" : undefined}
          {...register("email")}
        />
        {errors.email ? (
          <p id="email-error" className="text-sm text-destructive">
            {errors.email.message}
          </p>
        ) : null}
      </div>

      <div className="space-y-2">
        <Label htmlFor="password">Пароль</Label>
        <div className="relative">
          <Input
            id="password"
            type={showPassword ? "text" : "password"}
            autoComplete="current-password"
            className="pr-11"
            aria-invalid={errors.password ? true : undefined}
            aria-describedby={errors.password ? "password-error" : undefined}
            {...register("password")}
          />
          <button
            type="button"
            onClick={() => setShowPassword((v) => !v)}
            aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
            aria-pressed={showPassword}
            className="absolute right-1 top-1 grid h-8 w-8 place-items-center rounded text-muted-foreground hover:text-foreground"
          >
            {showPassword ? <EyeOff aria-hidden className="h-4 w-4" /> : <Eye aria-hidden className="h-4 w-4" />}
          </button>
        </div>
        {errors.password ? (
          <p id="password-error" className="text-sm text-destructive">
            {errors.password.message}
          </p>
        ) : null}
      </div>

      <Button type="submit" size="lg" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? <Loader2 aria-hidden className="h-4 w-4 animate-spin" /> : null}
        {isSubmitting ? "Вход…" : "Войти"}
      </Button>
    </form>
  );
}
