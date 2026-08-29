/**
 * Is it up — the three screens an operator opens at 3am.
 *
 * What these tests are mostly about is the difference between a number and no
 * number. A monitoring screen that prints 0.00% when it could not reach
 * Prometheus is worse than one that prints nothing: the first says the
 * deployment is healthy, and it is the screen somebody checks before going back
 * to bed.
 */

import { expect, test, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { anOverview } from "../helpers/fixtures";

const api = vi.hoisted(() => ({ health: vi.fn() }));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Metrics from "@/app/cp/ops/page";
import Alerts from "@/app/cp/ops/alerts/page";
import Jobs from "@/app/cp/ops/jobs/page";

test("the API's three numbers are blank when the API's numbers could not be read", async () => {
  api.health.mockResolvedValue(
    anOverview({
      monitoring: false,
      // What a deployment without Prometheus sends: zeroes, and `read: false`
      // to say they mean nothing.
      api: { requests_per_second: 0, error_rate: 0, p95_seconds: 0, read: false },
    }),
  );

  render(<Metrics />);

  // The banner across the top. Waited for by its own element rather than by
  // the text, which the two empty tables also carry before the answer arrives.
  await screen.findByText("cp.message.no_monitoring", { selector: "p" });
  // And once in each of the two tables that have nothing to put in them.
  expect(screen.getAllByText("cp.message.no_monitoring")).toHaveLength(3);
  // Three gauges, three dashes — not "0.0", "0.00%" and "0 ms", which is what
  // a healthy deployment at four in the morning also looks like.
  expect(screen.getAllByText("—")).toHaveLength(3);
});

test("an error rate over one percent is drawn as a problem", async () => {
  api.health.mockResolvedValue(
    anOverview({ api: { requests_per_second: 8, error_rate: 0.042, p95_seconds: 0.2, read: true } }),
  );

  render(<Metrics />);

  const rate = await screen.findByText("4.20%");
  expect(rate.className).toContain("text-red-600");
  expect(screen.getByText("8.0")).toBeTruthy();
  expect(screen.getByText("200 ms")).toBeTruthy();
});

test("Grafana is offered only where there is a Grafana", async () => {
  api.health.mockResolvedValue(anOverview({ grafana_url: "" }));
  const { unmount } = render(<Metrics />);
  await screen.findByText("cp.section.infrastructure");
  expect(screen.queryByRole("link", { name: /Grafana/ })).toBeNull();
  unmount();

  api.health.mockResolvedValue(anOverview({ grafana_url: "https://grafana.example.test" }));
  render(<Metrics />);

  const link = await screen.findByRole("link", { name: /Grafana/ });
  expect(link.getAttribute("href")).toBe("https://grafana.example.test");
  // A console tab opened from a link that can reach back into it is a console
  // tab somebody else's page can drive.
  expect(link.getAttribute("rel")).toContain("noopener");
});

test("refreshing asks again rather than redrawing what is already there", async () => {
  api.health.mockResolvedValue(anOverview());
  const person = userEvent.setup();

  render(<Metrics />);
  await screen.findByText("cp.section.external");
  expect(api.health).toHaveBeenCalledTimes(1);

  await person.click(screen.getByRole("button", { name: "cp.action.refresh" }));
  expect(api.health).toHaveBeenCalledTimes(2);
});

test("a monitoring screen that cannot reach the API says so", async () => {
  api.health.mockRejectedValue(new Error("prometheus is unreachable"));

  render(<Metrics />);

  expect(await screen.findByText("prometheus is unreachable")).toBeTruthy();
});

test("a firing alert carries its severity, its silence and its runbook", async () => {
  api.health.mockResolvedValue(
    anOverview({
      alerts: [
        {
          name: "ApiErrorRateHigh",
          severity: "critical",
          summary: "5xx over 2% for ten minutes",
          starts_at: "2026-08-29T03:10:00+08:00",
          runbook: "https://runbooks.example.test/api",
          silenced: true,
        },
      ],
    }),
  );

  render(<Alerts />);

  const name = await screen.findByText("ApiErrorRateHigh");
  expect(screen.getByText("critical")).toBeTruthy();
  expect(screen.getByText("5xx over 2% for ten minutes")).toBeTruthy();
  // Silenced is a property of the alert, not a reason to leave it out: an
  // operator reading this list needs to know it is firing *and* muted.
  expect(within(name.closest("td")!).getByLabelText("cp.state.silenced")).toBeTruthy();
  expect(screen.getByRole("link", { name: /cp\.action\.runbook/ }).getAttribute("href")).toBe(
    "https://runbooks.example.test/api",
  );
});

test("alerts and configuration warnings are two lists with two empty states", async () => {
  api.health.mockResolvedValue(anOverview({ alerts: [], warnings: [] }));
  const { unmount } = render(<Alerts />);

  expect(await screen.findByText("cp.message.nothing_firing")).toBeTruthy();
  expect(screen.getByText("cp.message.no_warnings")).toBeTruthy();
  unmount();

  // A warning is the platform saying it was configured in a way that will go
  // wrong later; nothing is firing, and the screen must not read as if
  // something is.
  api.health.mockResolvedValue(anOverview({ alerts: [], warnings: ["SMTP is not configured"] }));
  render(<Alerts />);

  expect(await screen.findByText("SMTP is not configured")).toBeTruthy();
  expect(screen.getByText("cp.message.nothing_firing")).toBeTruthy();
  expect(screen.queryByText("cp.message.no_warnings")).toBeNull();
});

test("a job that has never run and a job that is failing read differently", async () => {
  api.health.mockResolvedValue(
    anOverview({
      background: [
        { name: "catalogue sync", last_run: null, ok: true, detail: "", pending: 0 },
        { name: "scheduled reports", last_run: "2026-08-29T06:00:00+08:00", ok: false, detail: "smtp refused", pending: 3 },
      ],
    }),
  );

  render(<Jobs />);

  const never = (await screen.findByText("catalogue sync")).closest("tr")!;
  // No last run and nothing pending: two dashes, and the normal badge, because
  // a job with nothing to do is not a job in trouble.
  expect(within(never).getAllByText("—")).toHaveLength(2);
  expect(within(never).getByText("cp.state.normal")).toBeTruthy();

  const failing = screen.getByText("scheduled reports").closest("tr")!;
  expect(within(failing).getByText("cp.state.failing")).toBeTruthy();
  expect(within(failing).getByText("smtp refused")).toBeTruthy();
  expect(within(failing).getByText("3")).toBeTruthy();
});

test("organisations a job keeps failing for are named, not counted", async () => {
  api.health.mockResolvedValue(
    anOverview({
      background: [],
      tenant_trouble: [{ tenant_id: "ten-9", name: "Хан Уул ХХК", failures: 12, sample: "smtp: 550 mailbox unavailable" }],
    }),
  );

  render(<Jobs />);

  const row = (await screen.findByText("Хан Уул ХХК")).closest("tr")!;
  expect(within(row).getByText("12")).toBeTruthy();
  expect(within(row).getByText("smtp: 550 mailbox unavailable")).toBeTruthy();
  // The jobs table above it is empty and says so on its own.
  expect(screen.getByText("cp.message.no_activity")).toBeTruthy();
});
