import type { Metadata, Viewport } from "next";
import "./globals.css";

export const metadata: Metadata = { title: "IC Tender", description: "Закрытая B2B-тендерная платформа IC Group" };
export const viewport: Viewport = { themeColor: "#050c08", colorScheme: "dark" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru" className="dark">
      <body>{children}</body>
    </html>
  );
}
