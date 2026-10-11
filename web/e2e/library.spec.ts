import { expect, test } from "./fixtures/mock-client";

test.describe("library browser", () => {
  test("empty library shows the empty state", async ({ mainPage, mock }) => {
    await mock.seedLibrary({});
    await mainPage.open();

    await expect(mainPage.library.emptyMessage).toBeVisible();
    await expect(mainPage.library.upButton).toBeDisabled();
    await expect(mainPage.library.convertFolderButton).toBeHidden();
    await mainPage.screenshot("library-empty");
  });

  test("lists dirs first and navigates in and out", async ({ mainPage, mock }) => {
    await mock.seedLibrary({
      Movies: { "a.mkv": true, "b.mkv": true },
      Shows: { S01: { "e1.mkv": true } },
      "standalone.mp4": true,
    });
    await mainPage.open();
    const lib = mainPage.library;

    await expect(lib.rows).toHaveCount(3);
    // dirs sort before files
    await expect(lib.rows.nth(0)).toContainText("Movies");
    await expect(lib.rows.nth(1)).toContainText("Shows");
    await expect(lib.rows.nth(2)).toContainText("standalone.mp4");
    await expect(lib.row("Movies")).toHaveAttribute("data-dir", "true");
    await expect(lib.row("standalone.mp4")).toHaveAttribute("data-dir", "false");

    await lib.openDir("Movies");
    await expect(lib.crumbs).toContainText("Movies");
    await expect(lib.row("a.mkv")).toBeVisible();
    await expect(lib.row("b.mkv")).toBeVisible();
    await expect(lib.upButton).toBeEnabled();
    await expect(lib.convertFolderButton).toBeVisible();
    await mainPage.screenshot("library-folder");

    await lib.up();
    await expect(lib.crumbs).not.toContainText("Movies");
    await expect(lib.upButton).toBeDisabled();
    await expect(lib.convertFolderButton).toBeHidden();
  });

  test("double-click on a folder row also opens it", async ({ mainPage, mock }) => {
    await mock.seedLibrary({ Movies: { "a.mkv": true } });
    await mainPage.open();

    await mainPage.library.row("Movies").dblclick();
    await expect(mainPage.library.crumbs).toContainText("Movies");
    await expect(mainPage.library.row("a.mkv")).toBeVisible();
  });

  test("refresh picks up backend changes, but not silently", async ({ mainPage, mock }) => {
    await mock.seedLibrary({ Movies: { "a.mkv": true } });
    await mainPage.open();
    const lib = mainPage.library;

    await lib.openDir("Movies");
    await expect(lib.row("a.mkv")).toBeVisible();

    await mock.seedLibrary({ Movies: { "a.mkv": true, "new.mkv": true } });
    await expect(lib.row("new.mkv")).toBeHidden(); // entries are only loaded on demand

    await lib.refresh();
    await expect(lib.row("new.mkv")).toBeVisible();
  });

  test("file rows offer Convert; folder rows do not", async ({ mainPage, mock }) => {
    await mock.seedLibrary({ Movies: { "a.mkv": true }, "standalone.mp4": true });
    await mainPage.open();
    const lib = mainPage.library;

    await expect(
      lib.row("standalone.mp4").getByRole("button", { name: "Convert", exact: true }),
    ).toBeVisible();
    await expect(
      lib.row("Movies").getByRole("button", { name: "Convert", exact: true }),
    ).toBeHidden();
  });
});
