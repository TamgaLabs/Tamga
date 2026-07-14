import type { Metadata } from "next";
import { GeistSans } from "geist/font/sans";
import { GeistMono } from "geist/font/mono";
import { GeistPixelSquare } from "geist/font/pixel";
import "./globals.css";
import { AuthProvider } from "@/lib/auth";
import { ThemeProvider } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { OfflinePreviewBanner } from "@/components/offline-preview-banner";
import { Toaster } from "@/components/ui/sonner";

const geistSans = GeistSans;
const geistMono = GeistMono;
const geistPixel = GeistPixelSquare;

export const metadata: Metadata = {
  title: "Tamga Console",
  description: "Tamga Console — infrastructure and project operations.",
  icons: {
    icon: "/favicon.ico",
    apple: "/apple-touch-icon.png",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning className={cn(geistSans.variable, geistMono.variable, geistPixel.variable)}>
      <body className="min-h-screen overflow-x-hidden font-sans antialiased">
        <ThemeProvider>
          <AuthProvider>{children}</AuthProvider>
          <OfflinePreviewBanner />
          <Toaster richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
