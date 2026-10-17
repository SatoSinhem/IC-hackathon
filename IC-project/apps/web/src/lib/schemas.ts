import { z } from "zod";

export const PASSWORD_MIN = 12;
export const PASSWORD_MAX = 128;

export const loginSchema = z.object({
  email: z.string().trim().min(1, "Укажите email").email("Введите корректный email"),
  password: z.string().min(1, "Укажите пароль"),
});

export const changePasswordSchema = z
  .object({
    currentPassword: z.string().min(1, "Укажите текущий пароль"),
    newPassword: z
      .string()
      .min(PASSWORD_MIN, `Не короче ${PASSWORD_MIN} символов`)
      .max(PASSWORD_MAX, `Не длиннее ${PASSWORD_MAX} символов`),
    confirmPassword: z.string().min(1, "Повторите новый пароль"),
  })
  .refine((v) => v.newPassword === v.confirmPassword, { path: ["confirmPassword"], message: "Пароли не совпадают" })
  .refine((v) => v.newPassword !== v.currentPassword, { path: ["newPassword"], message: "Новый пароль должен отличаться от текущего" });

export type LoginValues = z.infer<typeof loginSchema>;
export type ChangePasswordValues = z.infer<typeof changePasswordSchema>;
