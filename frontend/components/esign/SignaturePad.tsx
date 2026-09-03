"use client";

import React, { useEffect, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";

/**
 * A canvas the signer draws on, used by the HSM rail.
 *
 * The canvas is sized from its own layout box rather than fixed attributes.
 * A CSS-stretched canvas keeps its intrinsic pixel grid, so strokes land at
 * the wrong place on any width but the one it was authored at, and the
 * exported PNG comes out blurry on a high-density display.
 */
export default function SignaturePad({
  onChange,
  disabled,
}: {
  onChange: (dataUrl: string | null) => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const drawing = useRef(false);
  const [hasInk, setHasInk] = useState(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const resize = () => {
      const ratio = window.devicePixelRatio || 1;
      const rect = canvas.getBoundingClientRect();
      if (!rect.width) return;
      canvas.width = Math.round(rect.width * ratio);
      canvas.height = Math.round(rect.height * ratio);
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      // Draw in CSS pixels; the transform handles the device ratio.
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
      ctx.lineWidth = 2.5;
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
      ctx.strokeStyle = "#1e293b";
    };

    resize();
    // Resizing clears the bitmap, so a signature drawn before a rotation would
    // silently vanish. Report that rather than submitting an empty stamp.
    const observer = new ResizeObserver(() => {
      resize();
      if (hasInk) {
        setHasInk(false);
        onChange(null);
      }
    });
    observer.observe(canvas);
    return () => observer.disconnect();
  }, [hasInk, onChange]);

  const positionOf = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };

  const start = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (disabled) return;
    const ctx = canvasRef.current?.getContext("2d");
    if (!ctx) return;
    // Capturing the pointer keeps a stroke attached to the canvas when the
    // finger slides past its edge mid-signature.
    event.currentTarget.setPointerCapture(event.pointerId);
    drawing.current = true;
    const { x, y } = positionOf(event);
    ctx.beginPath();
    ctx.moveTo(x, y);
  };

  const move = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current || disabled) return;
    const ctx = canvasRef.current?.getContext("2d");
    if (!ctx) return;
    const { x, y } = positionOf(event);
    ctx.lineTo(x, y);
    ctx.stroke();
    if (!hasInk) setHasInk(true);
  };

  const end = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    drawing.current = false;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    const canvas = canvasRef.current;
    if (canvas && hasInk) onChange(canvas.toDataURL("image/png"));
  };

  const clear = () => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    // Cleared in device pixels; the transform is on the context, not the reset.
    ctx.save();
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.restore();
    setHasInk(false);
    onChange(null);
  };

  return (
    <div className={disabled ? "opacity-40 pointer-events-none" : ""}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-semibold text-foreground uppercase tracking-wide">
          {t("esign.view.step_signature")}
        </span>
        <button
          type="button"
          onClick={clear}
          className="text-xs text-muted hover:text-foreground underline"
        >
          {t("esign.action.clear")}
        </button>
      </div>
      <canvas
        ref={canvasRef}
        onPointerDown={start}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={end}
        aria-label={t("esign.view.step_signature")}
        className="w-full h-40 border-2 border-dashed border-input rounded-lg bg-surface-2 touch-none cursor-crosshair"
      />
    </div>
  );
}
