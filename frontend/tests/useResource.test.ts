import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useResource } from "@/lib/useResource";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

describe("useResource", () => {
  it("keeps the newest result when an older request finishes last", async () => {
    const old = deferred<string>();
    const fresh = deferred<string>();
    const load = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const { result } = renderHook(() => useResource(load, { initial: "" }));
    let reload!: Promise<void>;
    act(() => { reload = result.current.reload(); });
    await act(async () => { fresh.resolve("new"); await reload; });
    await act(async () => { old.resolve("old"); await old.promise; });
    expect(result.current.data).toBe("new");
    expect(result.current.loading).toBe(false);
  });

  it("does not end loading or report errors from a superseded request", async () => {
    const old = deferred<string>();
    const fresh = deferred<string>();
    const onError = vi.fn();
    const load = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const { result } = renderHook(() => useResource(load, { initial: "", onError }));
    let reload!: Promise<void>;
    act(() => { reload = result.current.reload(); });
    await act(async () => { old.reject(new Error("old failure")); });
    expect(result.current.loading).toBe(true);
    expect(result.current.failed).toBe(false);
    expect(onError).not.toHaveBeenCalled();
    await act(async () => { fresh.resolve("new"); await reload; });
    expect(result.current.data).toBe("new");
  });

  it("does not report an error after the screen unmounts", async () => {
    const pending = deferred<string>();
    const onError = vi.fn();
    const { unmount } = renderHook(() => useResource(() => pending.promise, { initial: "", onError }));
    unmount();
    await act(async () => { pending.reject(new Error("late failure")); });
    expect(onError).not.toHaveBeenCalled();
  });

  it("reports current failures and recovers on reload", async () => {
    const pending = deferred<string>();
    const onError = vi.fn();
    const load = vi.fn().mockReturnValueOnce(pending.promise).mockResolvedValueOnce("recovered");
    const { result } = renderHook(() => useResource(load, { initial: "", onError }));
    const error = new Error("current failure");
    await act(async () => { pending.reject(error); });
    expect(result.current.failed).toBe(true);
    expect(result.current.loading).toBe(false);
    expect(onError).toHaveBeenCalledWith(error);
    await act(async () => { await result.current.reload(); });
    expect(result.current.data).toBe("recovered");
    expect(result.current.failed).toBe(false);
  });
});
