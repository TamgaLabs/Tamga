"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { useTheme, type Theme } from "@/lib/theme";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";

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
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Appearance</PageHeaderTitle>
          <PageHeaderDescription>Set the console theme and choose which system resources stay visible.</PageHeaderDescription>
        </div>
      </PageHeader>

      <div className="grid gap-4">
        <Card>
          <CardHeader className="space-y-1">
            <CardTitle>Theme</CardTitle>
            <p className="text-sm text-muted-foreground">Choose how Tamga Console appears on this device.</p>
          </CardHeader>
          <CardContent>
            <RadioGroup
              value={theme}
              onValueChange={(v) => setTheme(v as Theme)}
              className="space-y-2"
            >
              <div className="flex items-center gap-3 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                <RadioGroupItem value="light" id="theme-light" />
                <Label htmlFor="theme-light">Light</Label>
              </div>
              <div className="flex items-center gap-3 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                <RadioGroupItem value="dark" id="theme-dark" />
                <Label htmlFor="theme-dark">Dark</Label>
              </div>
              <div className="flex items-center gap-3 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
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
          <CardHeader className="space-y-1">
            <CardTitle>Display</CardTitle>
            <p className="text-sm text-muted-foreground">Control whether internal Tamga resources appear throughout the console.</p>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
              <div className="space-y-1">
                <Label htmlFor="show-system">Show Tamga System</Label>
                <p className="text-sm text-muted-foreground">Tamga system containers and codebases appear across the console.</p>
              </div>
              <Switch
                id="show-system"
                checked={showSystemState}
                onCheckedChange={handleToggleSystem}
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
