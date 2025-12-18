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
import { Kbd } from "@/components/ui/kbd";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { sendHttpRequest } from "./http";
import { handleWebSocket } from "./websocket";
import { cache, Result } from "./utils";

// Updated ApiPlayground component to use modularized logic
export default function ApiPlayground() {
  const presets = [
    {
      label: "Get user",
      url: "https://tether.eggwite.moe/v1/users/{user_id}",
      type: "http",
    },
    {
      label: "Get server health",
      url: "https://tether.eggwite.moe/healthz",
      type: "http",
    },
    {
      label: "WS Gateway",
      url: "",
      type: "ws",
    },
  ];

  const [selectedPresetIndex, setSelectedPresetIndex] = useState<number | null>(
    0
  );
  const [url, setUrl] = useState(
    "https://tether.eggwite.moe/v1/users/{user_id}"
  );
  const [protocol, setProtocol] = useState("http");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<number | null>(null);

  // Load cached value when URL changes (stale-while-revalidate)
  useEffect(() => {
    if (!url) {
      return; // Do not reset result to null
    }
    const cached = cache.get(url);
    // Only populate from cache when we don't already have a result
    if (cached) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setResult((prev) => prev ?? cached);
    }
  }, [url]);

  // Dedicated WebSocket endpoint (use /ws path used in docs/screenshots)
  const WS_ENDPOINT = "wss://tether.eggwite.moe/socket";

  const handleSend = useCallback(() => {
    if (protocol === "http") {
      // let the HTTP helper set the result directly
      sendHttpRequest(url, setResult, setLoading);
      return;
    }

    // let the WebSocket helper set the result directly
    handleWebSocket(url, setResult, wsRef, heartbeatRef, WS_ENDPOINT);
  }, [protocol, url]);

  // Enter key submits
  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSend();
    }
  };

  // Cleanup WS and heartbeat on unmount
  useEffect(() => {
    return () => {
      try {
        if (heartbeatRef.current) {
          window.clearInterval(heartbeatRef.current);
          heartbeatRef.current = null;
        }
        if (wsRef.current) {
          wsRef.current.close();
          wsRef.current = null;
        }
      } catch {}
    };
  }, []);

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
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(220px,1fr)_auto] md:grid-cols-[minmax(220px,1fr)_auto] items-center">
          <div className="flex gap-2 items-center w-full">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline">
                  Presets
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                {presets.map((p, i) => (
                  <DropdownMenuItem
                    key={p.url}
                    onClick={() => {
                      setSelectedPresetIndex(i);
                      setProtocol(p.type); // Update protocol based on preset
                      // For WS presets the input is used for IDs; clear it.
                      if (p.type === "ws") setUrl("");
                      else setUrl(p.url);
                    }}
                  >
                    {p.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>

            <Select value={protocol} onValueChange={setProtocol}>
              <SelectTrigger className="w-45">
                <SelectValue placeholder="Protocol" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="http">HTTP</SelectItem>
                <SelectItem value="ws">WebSocket</SelectItem>
              </SelectContent>
            </Select>

            <Input
              placeholder={
                protocol === "ws"
                  ? "Input IDs comma separated"
                  : "https://api.example.com/endpoint"
              }
              value={url}
              onChange={(e: any) => setUrl(e.target.value)}
              onKeyDown={onKeyDown}
              className="w-full"
              aria-label={protocol === "ws" ? "User IDs input" : "Request URL"}
            />
          </div>

          <div className="flex items-center gap-2 w-full justify-end">
            <Button onClick={handleSend} disabled={loading} size="sm">
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
