import { describe, expect, it } from "vitest";
import { formatDuration } from "./format";

describe("formatDuration", () => {
  it("formats seconds as m:ss", () => {
    expect(formatDuration(0)).toBe("0:00");
    expect(formatDuration(5000)).toBe("0:05");
    expect(formatDuration(65000)).toBe("1:05");
  });

  it("switches to h:mm:ss over an hour", () => {
    expect(formatDuration(3600_000)).toBe("1:00:00");
    expect(formatDuration(3723_000)).toBe("1:02:03");
  });

  it("drops sub-second parts and clamps negatives", () => {
    expect(formatDuration(1599)).toBe("0:01");
    expect(formatDuration(-5000)).toBe("0:00");
  });
});
