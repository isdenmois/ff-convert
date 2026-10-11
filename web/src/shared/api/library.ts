import { api } from "./client";

export interface LibraryEntry {
  name: string;
  path: string;
  dir: boolean;
}

export function listLibrary(path: string): Promise<LibraryEntry[]> {
  return api("GET", `/api/library?path=${encodeURIComponent(path)}`);
}
