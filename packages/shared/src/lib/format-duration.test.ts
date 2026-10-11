import { describe, expect, it } from "vitest";
import { formatDuration } from "./format-duration";

describe("formatDuration", () => {
  it('should format zero as "0:00"', () => {
    // Arrange
    const ms = 0;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("0:00");
  });

  it("should zero-pad seconds below ten", () => {
    // Arrange
    const ms = 5000;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("0:05");
  });

  it("should format minutes and seconds as m:ss", () => {
    // Arrange
    const ms = 65000;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("1:05");
  });

  it("should switch to h:mm:ss at exactly one hour", () => {
    // Arrange
    const ms = 3_600_000;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("1:00:00");
  });

  it("should zero-pad minutes and seconds over an hour", () => {
    // Arrange
    const ms = 3_723_000;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("1:02:03");
  });

  it("should drop sub-second parts", () => {
    // Arrange
    const ms = 1599;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("0:01");
  });

  it("should clamp negative durations to zero", () => {
    // Arrange
    const ms = -5000;

    // Act
    const label = formatDuration(ms);

    // Assert
    expect(label).toBe("0:00");
  });
});
