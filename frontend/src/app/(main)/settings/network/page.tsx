import { redirect } from "next/navigation";

export default function NetworkSettingsPage() {
  redirect("/settings#network");
}
