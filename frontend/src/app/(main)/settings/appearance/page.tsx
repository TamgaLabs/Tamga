"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { useTheme, type Theme } from "@/lib/theme";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

export default function AppearanceSettingsPage() {
  const [showSystemState, setShowSystemState] = useState(true);
  const { user, loading: authLoading } = useAuth();
  const { theme, setTheme } = useTheme();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    setShowSystemState(getShowSystem());
  }, [user]);

  const handleToggleSystem = () => {
    const next = !showSystemState;
    setShowSystemState(next);
    setShowSystem(next);
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Appearance</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Theme</CardTitle>
          </CardHeader>
          <CardContent>
            <RadioGroup
              value={theme}
              onValueChange={(v) => setTheme(v as Theme)}
              className="space-y-2"
            >
              <div className="flex items-center gap-2">
                <RadioGroupItem value="light" id="theme-light" />
                <Label htmlFor="theme-light">Light</Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="dark" id="theme-dark" />
                <Label htmlFor="theme-dark">Dark</Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="system" id="theme-system" />
                <Label htmlFor="theme-system">System</Label>
              </div>
            </RadioGroup>
            <p className="text-xs text-muted-foreground mt-3">
              System follows your OS preference and updates live if it changes.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Display</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Checkbox
                id="show-system"
                checked={showSystemState}
                onCheckedChange={handleToggleSystem}
              />
              <Label htmlFor="show-system">Show Tamga System</Label>
            </div>
            <p className="text-xs text-muted-foreground mt-2">
              When disabled, Tamga system containers and codebases are hidden from all pages.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
