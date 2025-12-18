// Shared utilities for the API Playground
export type Result = {
  status?: number;
  headers?: [string, string][];
  durationMs?: number;
  body?: any;
  error?: string;
};

// Simple in-memory cache for SWR-like behavior across component mounts.
export const cache = new Map<string, Result>();