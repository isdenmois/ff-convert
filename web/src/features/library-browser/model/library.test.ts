import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryEntry } from "@/shared/api/library";
import { listLibrary } from "@/shared/api/library";
import { useLibraryStore } from "./library";

vi.mock("@/shared/api/library", () => ({
  listLibrary: vi.fn(),
}));

function dir(name: string): LibraryEntry {
  return { name, path: `/lib/${name}`, dir: true };
}

describe("library store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(listLibrary).mockReset();
  });

  it("should load the entries for the current path", async () => {
    // Arrange
    const entry = { name: "a.mkv", path: "/lib/a.mkv", dir: false };
    vi.mocked(listLibrary).mockResolvedValue([entry]);
    const store = useLibraryStore();

    // Act
    await store.load();

    // Assert
    expect(listLibrary).toHaveBeenCalledWith("");
    expect(store.entries).toEqual([entry]);
    expect(store.loading).toBe(false);
    expect(store.error).toBe("");
  });

  it("should record the error when loading fails", async () => {
    // Arrange
    vi.mocked(listLibrary).mockRejectedValue(new Error("boom"));
    const store = useLibraryStore();

    // Act
    await store.load();

    // Assert
    expect(store.error).not.toBe("");
    expect(store.loading).toBe(false);
  });

  it("should navigate into a directory and reload its entries", () => {
    // Arrange
    vi.mocked(listLibrary).mockResolvedValue([]);
    const store = useLibraryStore();

    // Act
    store.open(dir("movies"));

    // Assert
    expect(store.path).toBe("/lib/movies");
    expect(listLibrary).toHaveBeenCalledWith("/lib/movies");
  });

  it("should pop one path segment on up", () => {
    // Arrange
    vi.mocked(listLibrary).mockResolvedValue([]);
    const store = useLibraryStore();
    store.path = "movies/scifi";

    // Act
    store.up();

    // Assert
    expect(store.path).toBe("movies");
    expect(listLibrary).toHaveBeenCalledWith("movies");
  });

  it("should stay at the root when already there", () => {
    // Arrange
    vi.mocked(listLibrary).mockResolvedValue([]);
    const store = useLibraryStore();

    // Act
    store.up();

    // Assert
    expect(store.path).toBe("");
    expect(listLibrary).toHaveBeenCalledWith("");
  });
});
