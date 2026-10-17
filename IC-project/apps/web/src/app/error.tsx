"use client";

import { Button } from "@/components/ui/button";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="max-w-md text-center">
        <h1 className="text-2xl font-semibold">Не удалось загрузить страницу</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Сервер временно недоступен. Убедитесь, что API запущен и подключён к базе данных, затем повторите попытку.
        </p>
        <Button className="mt-6" onClick={reset}>
          Повторить
        </Button>
      </div>
    </main>
  );
}
