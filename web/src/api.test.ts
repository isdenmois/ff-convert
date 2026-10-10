import { afterEach, describe, expect, it, vi } from "vitest";
import { cancelJob, createJob, removeJob, submitAnswers } from "./api";
import type { Answers } from "./types";

const fetchMock = vi.fn();
vi.stubGlobal("fetch", fetchMock);

function respondOk(body: unknown) {
  return {
    ok: true,
    statusText: "OK",
    json: () => Promise.resolve(body),
  };
}

afterEach(() => {
  fetchMock.mockReset();
});

describe("api", () => {
  it("createJob POSTs the path as JSON", async () => {
    const job = { id: "j1", name: "test" };
    fetchMock.mockResolvedValue(respondOk(job));

    await expect(createJob("/movies/file.mkv")).resolves.toEqual(job);
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: "/movies/file.mkv" }),
    });
  });

  it("submitAnswers POSTs the answers for the job id", async () => {
    const job = { id: "j1", state: "running" };
    fetchMock.mockResolvedValue(respondOk(job));
    const answers: Answers[] = [
      { audio: [0], subs: [1], preset: "copy", normalize: true, attachments: true },
    ];

    await expect(submitAnswers("j1", answers)).resolves.toEqual(job);
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1/answers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    });
  });

  it("cancelJob sends no body", async () => {
    fetchMock.mockResolvedValue(respondOk({ id: "j1" }));

    await cancelJob("j1");
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1/cancel", {
      method: "POST",
      headers: undefined,
      body: undefined,
    });
  });

  it("removeJob issues a DELETE", async () => {
    fetchMock.mockResolvedValue(respondOk({}));

    await removeJob("j1");
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1", {
      method: "DELETE",
      headers: undefined,
      body: undefined,
    });
  });

  it("throws the server-provided error message on failure", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ error: "boom" }),
    });

    await expect(createJob("/x")).rejects.toThrow("boom");
  });
});
