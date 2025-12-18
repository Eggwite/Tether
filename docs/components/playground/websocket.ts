// WebSocket logic for the API Playground
import { Result } from "./utils";

export function handleWebSocket(
  url: string,
  setResult: React.Dispatch<React.SetStateAction<Result | null>>,
  wsRef: React.RefObject<WebSocket | null>,
  heartbeatRef: React.RefObject<number | null>,
  WS_ENDPOINT: string
) {
  const ids = url
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  if (ids.length === 0) {
    setResult({
      error: "Please enter one or more user IDs (comma separated).",
    });
    return;
  }

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

  console.log("Opening WebSocket to", WS_ENDPOINT, "with ids:", ids);
  // Only set a minimal "connecting" result when we don't already have data,
  // so we don't overwrite an existing full payload and cause UI flicker.
  setResult((prev) => prev ?? { body: { status: "connecting" }, status: 0 });

  const ws = new WebSocket(WS_ENDPOINT);
  wsRef.current = ws;

  ws.onopen = () => {
    console.log("WS open, waiting for HELLO...");
    // Preserve any existing payload — only set an "open" placeholder if
    // nothing is already displayed.
    setResult((prev) => prev ?? { body: { status: "open" }, status: 0 });
  };

  // Update WebSocket message handling to directly set the full payload
  ws.onmessage = (ev) => {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch (e) {
      console.warn("Non-JSON WS message", ev.data);
      return;
    }

    const op = msg?.op;
    if (op === 1) {
      const intervalMs = msg?.d?.heartbeat_interval ?? 30000;
      if (heartbeatRef.current) window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = window.setInterval(() => {
        ws.send(JSON.stringify({ op: 3 }));
      }, intervalMs);

      try {
        ws.send(
          JSON.stringify({
            op: 2,
            d: { subscribe_to_ids: ids },
          })
        );
      } catch (e) {
        console.error("Failed to send INITIALIZE", e);
      }
      return;
    }

    if (op === 0) {
      // Directly set the full payload without stripping. Use a direct
      // replacement so the UI shows the exact server message.
      setResult({ body: msg, status: 200 });
      return;
    }

    console.debug("WS op:", op, msg);
  };

  ws.onerror = (err) => {
    console.error("WebSocket error", err);
    // Merge error onto existing result rather than replacing body.
    setResult((prev) => ({ ...(prev ?? {}), error: "WebSocket error" }));
  };

  ws.onclose = () => {
    if (heartbeatRef.current) {
      window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = null;
    }
    wsRef.current = null;
    console.log("WebSocket closed");

    // Attempt reconnection
    setTimeout(() => {
      console.log("Reconnecting WebSocket...");
      handleWebSocket(url, setResult, wsRef, heartbeatRef, WS_ENDPOINT);
    }, 5000);
  };
}