import { redirect } from "next/navigation";

export default function SandboxSettingsPage() {
  redirect("/settings#sandbox");
}
