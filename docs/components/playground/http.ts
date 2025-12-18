// HTTP request logic for the API Playground
import { Result, cache } from "./utils";

export async function sendHttpRequest(url: string, setResult: (result: Result | null) => void, setLoading: (loading: boolean) => void) {
  if (!url) return;

  // validate URL: must be absolute and use http or https
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      setResult({ error: "invalid url: protocol must be http or https" });
      return;
    }
  } catch (e: any) {
    setResult({ error: "invalid url" });
    return;
  }

  const controller = new AbortController();
  setLoading(true);

  const start = performance.now();
  try {
    const res = await fetch(url, {
      method: "GET",
      signal: controller.signal,
    });

    let text = "";
    if (res.body && (res.body as any).getReader) {
      const reader = (res.body as any).getReader();
      const dek = new TextDecoder();
      let done = false;
      while (!done) {
        const { value, done: d } = await reader.read();
        if (value) text += dek.decode(value, { stream: !d });
        done = d;
      }
    } else {
      text = await res.text();
    }

    const end = Math.round(performance.now() - start);

    const headers = Array.from(res.headers.entries());
    let body: any = text;
    try {
      body = JSON.parse(text);
    } catch {
      // keep as text
    }

    const out: Result = {
      status: res.status,
      headers,
      durationMs: end,
      body,
    };
    cache.set(url, out);
    setResult(out);
  } catch (err: any) {
    if (err?.name === "AbortError") return;
    setResult({ error: err?.message ?? String(err) });
  } finally {
    setLoading(false);
  }
}