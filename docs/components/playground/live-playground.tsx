"use client";
import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  useMemo,
} from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Send } from "lucide-react";
import { DynamicCodeBlock } from "fumadocs-ui/components/dynamic-codeblock";
import { Kbd, KbdGroup } from "@/components/ui/kbd";

type Result = {
  status?: number;
  headers?: [string, string][];
  durationMs?: number;
  body?: any;
  error?: string;
};

// Simple in-memory cache for SWR-like behavior across component mounts.
const cache = new Map<string, Result>();

export default function ApiPlayground() {
  const [url, setUrl] = useState(
    "https://tether.eggwite.moe/v1/users/{user_id}"
  );
  // always use GET for this playground
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Load cached value when URL changes (stale-while-revalidate)
  useEffect(() => {
    if (!url) {
      setResult(null);
      return;
    }
    const cached = cache.get(url);
    if (cached) setResult(cached);
  }, [url]);

  const send = useCallback(async () => {
    if (!url) return;
    // abort previous
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);

    const start = performance.now();
    try {
      const res = await fetch(url, {
        method: "GET",
        signal: controller.signal,
      });
      const durationMs = Math.round(performance.now() - start);
      const headers = Array.from(res.headers.entries());
      const text = await res.text();
      let body: any = text;
      try {
        body = JSON.parse(text);
      } catch {
        // leave as text
      }
      const out: Result = { status: res.status, headers, durationMs, body };
      cache.set(url, out);
      setResult(out);
    } catch (err: any) {
      if (err.name === "AbortError") return;
      setResult({ error: err.message ?? String(err) });
    } finally {
      setLoading(false);
    }
  }, [url]);

  // Enter key submits
  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void send();
    }
  };

  // Nicely formatted header list
  const renderHeaders = (h?: [string, string][]) => {
    if (!h || h.length === 0)
      return <div className="text-sm text-muted-foreground">No headers</div>;
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Header</TableHead>
            <TableHead className="text-right">Value</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {h.map(([k, v]) => (
            <TableRow key={k}>
              <TableCell className="font-mono text-xs text-muted-foreground truncate max-w-[40%]">
                {k}
              </TableCell>
              <TableCell className="text-right wrap-break-word">{v}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  };

  const payloadSize = useMemo(() => {
    if (!result?.body) return 0;
    const text =
      typeof result.body === "string"
        ? result.body
        : JSON.stringify(result.body);
    try {
      return new TextEncoder().encode(text).length;
    } catch {
      return text.length;
    }
  }, [result]);

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "—";
    const units = ["B", "KB", "MB", "GB"];
    let i = 0;
    let num = bytes;
    while (num >= 1024 && i < units.length - 1) {
      num /= 1024;
      i++;
    }
    return `${num.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
  };

  return (
    <Card className="w-full max-w-4xl my-8">
      <CardHeader>
        <CardTitle>API Playground</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_auto] md:grid-cols-[1fr_auto] items-center">
          <Input
            placeholder="https://api.example.com/endpoint"
            value={url}
            onChange={(e: any) => setUrl(e.target.value)}
            onKeyDown={onKeyDown}
            className="w-full"
            aria-label="Request URL"
          />

          <div className="flex items-center gap-2 w-full justify-end">
            <Button onClick={() => void send()} disabled={loading} size="sm">
              <Send className="-ml-1 mr-2 h-4 w-4" />
              Send
            </Button>
          </div>
        </div>

        <div className="mt-4">
          {/* status and timing row */}
          <div className="flex items-center justify-between pb-2">
            <div>
              <span className="text-sm text-muted-foreground">Status:</span>{" "}
              {result?.status != null ? (
                <span
                  className={
                    result.status >= 200 && result.status < 300
                      ? "text-green-600 font-medium"
                      : result.status >= 400 && result.status < 500
                      ? "text-red-600 font-medium"
                      : result.status >= 500
                      ? "text-orange-500 font-medium"
                      : "text-muted-foreground"
                  }
                >
                  {result.status}
                </span>
              ) : (
                <span className="text-muted-foreground">—</span>
              )}
            </div>

            <div className="text-sm text-muted-foreground">
              Speed:{" "}
              {result?.durationMs != null ? `${result.durationMs} ms` : "—"}
            </div>
          </div>

          {/* response editor-like box */}
          {loading && !result ? (
            <Skeleton className="h-60 w-full" />
          ) : result?.error ? (
            <DynamicCodeBlock lang="json" code={String(result.error)} />
          ) : (
            <DynamicCodeBlock
              lang="json"
              code={
                typeof result?.body === "string"
                  ? result?.body
                  : JSON.stringify(result?.body, null, 2) ?? ""
              }
            />
          )}

          {/* headers table */}
          <div className="mt-4">
            {loading && !result ? (
              <div className="space-y-2">
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-4 w-3/4" />
              </div>
            ) : (
              renderHeaders(result?.headers)
            )}
          </div>
        </div>
      </CardContent>
      <CardFooter>
        <div className="w-full flex items-center justify-between">
          <div className="text-sm text-muted-foreground">
            <Kbd>Enter</Kbd> to send request
          </div>
          <div className="text-sm text-muted-foreground">
            Payload: {formatBytes(payloadSize)}
          </div>
        </div>
      </CardFooter>
    </Card>
  );
}
