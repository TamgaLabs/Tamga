import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  experimental: {},
  env: {
    NEXT_PUBLIC_API_URL: process.env.DOMAIN
      ? `https://api.${process.env.DOMAIN}`
      : "/api",
  },
};

export default nextConfig;
