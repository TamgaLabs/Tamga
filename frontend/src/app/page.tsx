"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { isOfflineMode } from "@/lib/offline-api";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    if (isOfflineMode()) {
      router.replace("/dashboard");
      return;
    }
    const token = localStorage.getItem("token");
    router.replace(token ? "/dashboard" : "/login");
  }, [router]);

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="animate-pulse text-muted-foreground">Loading...</div>
    </div>
  );
}
