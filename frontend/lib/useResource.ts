"use client";

import { Dispatch, SetStateAction, useCallback, useEffect, useRef, useState } from "react";

/**
 * One screen's worth of server data: fetch it on mount, hold it, refetch on
 * demand, and remember whether the last attempt failed.
 *
 * Every list screen had written this out by hand, and the copies agreed on
 * everything that matters, including the part that is easy to get wrong: a load
 * that FAILED must not leave the screen holding an empty array, because "no
 * documents yet" is a claim about the tenant and a tenant with twelve contracts
 * in the queue would be told it has none. `failed` is what a screen renders
 * instead of that claim, and it is cleared only by a load that actually
 * succeeded.
 *
 * `setData` is exposed because these screens edit rows in place — a checkbox
 * toggled before Save is pressed — and refetching the table to show one keypress
 * would throw away the edits made to every other row.
 */
/**
 * Runs `load` once, when the component mounts.
 *
 * This is what the screens meant by `useEffect(() => { load(); }, [])`, which
 * the exhaustive-deps rule flagged in every one of them — correctly, because
 * `load` is written inline and closes over state that a later render replaces.
 * Listing it as a dependency is not the fix either: a new identity per render
 * would refetch on every render.
 *
 * So the callback is read through a ref, which is written after each render.
 * The effect then genuinely has no reactive dependencies rather than being told
 * to ignore the ones it has, and a reload triggered later runs the current
 * version of the callback rather than the one captured at mount.
 *
 * Use this only for the load a screen does on arrival. Anything that should
 * re-run when a value changes belongs in an effect that names that value.
 */
export function useLoadOnMount(load: () => void | Promise<void>): void {
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  });
  useEffect(() => {
    void loadRef.current();
  }, []);
}

export interface Resource<T> {
  data: T;
  loading: boolean;
  /** True when the last attempt threw. Distinct from "loaded, and empty". */
  failed: boolean;
  setData: Dispatch<SetStateAction<T>>;
  reload: () => Promise<void>;
}

export function useResource<T>(
  load: () => Promise<T>,
  options: { initial: T; onError?: (err: unknown) => void },
): Resource<T> {
  const [data, setData] = useState<T>(options.initial);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  // load and onError are read through refs rather than named as dependencies.
  // Callers write them inline, so they are a different function on every render;
  // as dependencies they would refetch on every render, and each fetch would
  // cause the next one. The refs are updated after every render, so `reload`
  // called from a button always runs the current version.
  const loadRef = useRef(load);
  const onErrorRef = useRef(options.onError);
  const requestIdRef = useRef(0);
  useEffect(() => {
    loadRef.current = load;
    onErrorRef.current = options.onError;
  });

  const reload = useCallback(async () => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const nextData = await loadRef.current();
      if (requestId !== requestIdRef.current) return;
      setData(nextData);
      setFailed(false);
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      setFailed(true);
      onErrorRef.current?.(err);
    } finally {
      if (requestId === requestIdRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const requests = requestIdRef;
    void reload();
    // Invalidate pending work on unmount and during Strict Mode effect replay.
    // Only the latest request may change this screen or report an error.
    return () => { ++requests.current; };
  }, [reload]);

  return { data, loading, failed, setData, reload };
}
