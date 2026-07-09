"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useContainerContext } from "./container-context";

export default function ContainerInspectPage() {
  const { container } = useContainerContext();

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Container Config</CardTitle>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-[70vh]">
            <pre className="bg-code-block rounded p-4 text-xs text-success overflow-auto font-mono whitespace-pre-wrap">
              {JSON.stringify(container, null, 2)}
            </pre>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}
