"use client";

import React, { useEffect, useState } from "react";
import { Move, RotateCcw, Save } from "lucide-react";
import { esign, type Placement } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Loading, PageHeader } from "@/components/ui";
import { Alert, Button, Input } from "@gerege-systems/ui";
import { Card, useErrorMessage } from "@/components/esign/shared";

/** A4 in PostScript points — the page the preview and the limits are drawn to. */
const A4_WIDTH = 595;
const A4_HEIGHT = 842;

/**
 * Where the stamp lands on the page.
 *
 * The eSign service measures from the TOP-LEFT corner, unlike the PDF
 * specification's bottom-left origin. The preview below is therefore drawn the
 * same way round, so the numbers in the form and the box on the page agree —
 * getting that backwards is how a signature ends up off the paper while the
 * document still reports itself signed.
 */
export default function EsignPlacementPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [placement, setPlacement] = useState<Placement | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    esign
      .settings()
      .then((settings) => setPlacement(settings.placement))
      .catch((err) => setError(describe(err, t("base.message.error"))))
      .finally(() => setLoading(false));
  }, [describe, t]);

  const update = (patch: Partial<Placement>) =>
    setPlacement((current) => (current ? { ...current, ...patch } : current));

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!placement) return;
    setSaving(true);
    setError(null);
    try {
      setPlacement(await esign.savePlacement(placement));
      setNotice(t("esign.message.placement_saved"));
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Loading />;
  if (!placement) return <Alert variant="danger" live>{error ?? t("base.message.error")}</Alert>;

  const offPage =
    placement.x + placement.width > A4_WIDTH || placement.y + placement.height > A4_HEIGHT;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Move className="w-7 h-7 text-accent" />}
        title={t("esign.view.placement_title")}
        subtitle={t("esign.view.placement_subtitle")}
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}
      {notice && <Alert variant="success" live dismissible onDismiss={() => setNotice(null)}>{notice}</Alert>}
      {offPage && <Alert variant="danger" live>{t("esign.message.placement_off_page")}</Alert>}

      <div className="grid lg:grid-cols-2 gap-6 items-start">
        <Card title={t("esign.view.placement_form")}>
          <form onSubmit={save} className="p-4 space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <NumberField
                id="place-x"
                label={t("esign.field.x")}
                hint={t("esign.field.x_hint")}
                value={placement.x}
                min={0}
                max={A4_WIDTH}
                onChange={(x) => update({ x })}
              />
              <NumberField
                id="place-y"
                label={t("esign.field.y")}
                hint={t("esign.field.y_hint")}
                value={placement.y}
                min={0}
                max={A4_HEIGHT}
                onChange={(y) => update({ y })}
              />
              <NumberField
                id="place-w"
                label={t("esign.field.width")}
                value={placement.width}
                min={40}
                max={A4_WIDTH}
                onChange={(width) => update({ width })}
              />
              <NumberField
                id="place-h"
                label={t("esign.field.height")}
                value={placement.height}
                min={20}
                max={A4_HEIGHT}
                onChange={(height) => update({ height })}
              />
            </div>

            <Input
              id="place-page"
              type="number"
              min={0}
              label={t("esign.field.page_number")}
              helperText={t("esign.field.page_number_hint")}
              value={placement.page_number}
              onChange={(event) => update({ page_number: Number(event.target.value) })}
            />

            <Input
              id="place-text"
              label={t("esign.field.caption")}
              value={placement.text}
              maxLength={120}
              onChange={(event) => update({ text: event.target.value })}
            />

            <div className="flex gap-2 pt-1">
              <Button
                type="button"
                variant="outline"
                leadingIcon={<RotateCcw />}
                onClick={() =>
                  setPlacement({ x: 80, y: 216, width: 200, height: 56, page_number: 0, text: "Тоон гарын үсгээр баталгаажив." })
                }
              >
                {t("esign.action.reset_default")}
              </Button>
              <Button type="submit" loading={saving} disabled={offPage} leadingIcon={<Save />}>
                {saving ? t("base.message.saving") : t("base.action.save")}
              </Button>
            </div>
          </form>
        </Card>

        <Card title={t("esign.view.placement_preview")}>
          <div className="p-4">
            <div
              className="relative mx-auto bg-surface border border-input shadow-inner"
              style={{ width: "100%", maxWidth: 320, aspectRatio: `${A4_WIDTH} / ${A4_HEIGHT}` }}
            >
              {/* Percentages keep the preview faithful at any rendered width. */}
              <div
                className="absolute bg-accent-soft border-2 border-dashed border-accent flex items-end justify-center"
                style={{
                  left: `${(placement.x / A4_WIDTH) * 100}%`,
                  top: `${(placement.y / A4_HEIGHT) * 100}%`,
                  width: `${(placement.width / A4_WIDTH) * 100}%`,
                  height: `${(placement.height / A4_HEIGHT) * 100}%`,
                }}
              >
                <span className="text-xs text-accent font-semibold truncate px-1 pb-0.5">
                  {placement.text}
                </span>
              </div>
            </div>
            <p className="text-xs text-muted mt-3 text-center">
              {t("esign.message.placement_preview_hint", { width: A4_WIDTH, height: A4_HEIGHT })}
            </p>
          </div>
        </Card>
      </div>
    </div>
  );
}

function NumberField({
  id,
  label,
  hint,
  value,
  min,
  max,
  onChange,
}: {
  id: string;
  label: string;
  hint?: string;
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
}) {
  return (
    <Input
      id={id}
      type="number"
      min={min}
      max={max}
      label={label}
      helperText={hint}
      value={value}
      onChange={(event) => onChange(Number(event.target.value))}
    />
  );
}
