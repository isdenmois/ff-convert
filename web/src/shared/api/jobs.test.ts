import type { Answers } from "@ffconvert/shared/api";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cancelJob, createJob, removeJob, submitAnswers } from "./jobs";

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

describe("jobs api", () => {
  it("should POST the path as JSON when creating a job", async () => {
    // Arrange
    const job = { id: "j1", name: "test" };
    fetchMock.mockResolvedValue(respondOk(job));

    // Act
    const created = await createJob("/movies/file.mkv");

    // Assert
    expect(created).toEqual(job);
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: "/movies/file.mkv" }),
    });
  });

  it("should POST the answers for the job id when submitting answers", async () => {
    // Arrange
    const job = { id: "j1", state: "running" };
    const answers: Answers[] = [
      { audio: [0], subs: [1], preset: "copy", normalize: true, attachments: true },
    ];
    fetchMock.mockResolvedValue(respondOk(job));

    // Act
    const submitted = await submitAnswers("j1", answers);

    // Assert
    expect(submitted).toEqual(job);
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1/answers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    });
  });

  it("should send no body when canceling a job", async () => {
    // Arrange
    fetchMock.mockResolvedValue(respondOk({ id: "j1" }));

    // Act
    await cancelJob("j1");

    // Assert
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1/cancel", {
      method: "POST",
      headers: undefined,
      body: undefined,
    });
  });

  it("should issue a DELETE when removing a job", async () => {
    // Arrange
    fetchMock.mockResolvedValue(respondOk({}));

    // Act
    await removeJob("j1");

    // Assert
    expect(fetchMock).toHaveBeenCalledWith("/api/jobs/j1", {
      method: "DELETE",
      headers: undefined,
      body: undefined,
    });
  });

  it("should throw the server-provided error message when the request fails", async () => {
    // Arrange
    fetchMock.mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ error: "boom" }),
    });

    // Act
    const creating = createJob("/x");

    // Assert
    await expect(creating).rejects.toThrow("boom");
  });
});
