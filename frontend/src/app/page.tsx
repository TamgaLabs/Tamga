"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { checkSetup } from "@/lib/api";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    checkSetup()
      .then((res) => {
        if (res.setup) {
          const token = localStorage.getItem("token");
          router.replace(token ? "/dashboard" : "/login");
        } else {
          router.replace("/setup");
        }
      })
      .catch(() => router.replace("/setup"));
  }, [router]);

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="animate-pulse text-neutral-500">Loading...</div>
    </div>
  );
}
