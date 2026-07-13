"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { KeyRound, LockKeyhole, TerminalSquare } from "lucide-react";

import { login } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const { login: setAuth } = useAuth();
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      const res = await login(password);
      // Establish the user before navigating so the dashboard guard never
      // observes a token without its authenticated user state.
      await setAuth(res.token);
      router.replace("/dashboard");
    } catch {
      setError("Invalid password");
    }
  };

  return (
    <main className="grid min-h-svh bg-muted/30 lg:grid-cols-2">
      <section className="flex items-center justify-center px-4 py-10 sm:px-6">
        <Card className="w-full max-w-md border-border/80 shadow-lg shadow-primary/5">
          <CardHeader className="space-y-3 pb-7">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <TerminalSquare className="size-5" aria-hidden="true" />
            </div>
            <div className="space-y-1.5">
              <p className="font-display text-sm tracking-wide text-primary">Tamga Console</p>
              <CardTitle className="text-2xl">Welcome back</CardTitle>
              <CardDescription>Enter your admin password to continue.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-6">
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="password">Admin password</FieldLabel>
                  <InputGroup>
                    <InputGroupAddon aria-hidden="true"><LockKeyhole className="size-4" /></InputGroupAddon>
                    <InputGroupInput
                      id="password"
                      type="password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Enter password"
                      autoComplete="current-password"
                      aria-invalid={Boolean(error)}
                      aria-describedby={error ? "login-error" : undefined}
                      required
                    />
                  </InputGroup>
                  {error && <FieldError id="login-error">{error}</FieldError>}
                </Field>
              </FieldGroup>
              <Button type="submit" className="w-full">
                <KeyRound className="size-4" aria-hidden="true" />
                Sign in to console
              </Button>
            </form>
          </CardContent>
        </Card>
      </section>

      <section className="hidden border-l border-border bg-sidebar p-10 lg:flex lg:flex-col lg:justify-between">
        <div className="flex items-center gap-3 text-sidebar-foreground">
          <div className="flex size-9 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
            <TerminalSquare className="size-5" aria-hidden="true" />
          </div>
          <span className="font-display text-sm tracking-wide">Tamga Console</span>
        </div>
        <div className="max-w-md space-y-5">
          <p className="font-display text-xl leading-relaxed text-sidebar-foreground">OPERATE WITH CLARITY</p>
          <p className="text-sm leading-6 text-muted-foreground">
            One focused workspace for your projects, containers, and infrastructure.
          </p>
        </div>
        <p className="text-xs text-muted-foreground">Secure admin workspace</p>
      </section>
    </main>
  );
}
