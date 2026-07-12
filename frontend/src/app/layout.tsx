import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { AuthProvider } from "@/lib/auth";
import { ThemeProvider } from "@/lib/theme";
import { OfflinePreviewBanner } from "@/components/offline-preview-banner";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Tamga",
  description: "DevOps Automation Panel",
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
    <html lang="en" suppressHydrationWarning>
      <body className={`${inter.className} antialiased min-h-screen`}>
        <ThemeProvider>
          <AuthProvider>{children}</AuthProvider>
          <OfflinePreviewBanner />
        </ThemeProvider>
      </body>
    </html>
  );
}
