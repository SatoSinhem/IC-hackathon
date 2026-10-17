import Link from "next/link";
import { cn } from "@/lib/utils";

export function Brand({ href = "/", className }: { href?: string; className?: string }) {
  return (
    <Link href={href} className={cn("inline-flex items-center gap-2.5 rounded-md", className)} aria-label="IC Tender — на главную">
      <span aria-hidden className="grid h-9 w-9 place-items-center rounded-lg bg-primary text-sm font-black tracking-tight text-primary-foreground shadow-glow">
        IC
      </span>
      <span className="leading-tight">
        <span className="block text-base font-semibold">IC Tender</span>
        <span className="block text-[11px] uppercase tracking-[0.18em] text-muted-foreground">IC Group</span>
      </span>
    </Link>
  );
}
