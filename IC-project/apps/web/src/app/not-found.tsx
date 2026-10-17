import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export default function NotFound() {
  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="text-center">
        <p className="text-sm font-medium text-primary">404</p>
        <h1 className="mt-2 text-2xl font-semibold">Страница не найдена</h1>
        <p className="mt-2 text-sm text-muted-foreground">Проверьте адрес или вернитесь на главную.</p>
        <Link href="/" className={cn(buttonVariants(), "mt-6")}>
          На главную
        </Link>
      </div>
    </main>
  );
}
