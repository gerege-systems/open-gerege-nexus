import { describe, expect, it } from "vitest";

import { deviceLineFromHost, lineHomePath } from "@/lib/deviceLine";

/**
 * Шугамыг host-оос таних.
 *
 * Энэ таних үйлдэл нь middleware-ийн эхний алхам: буруу таньсан тохиолдолд
 * ажлын муж огт буруу нүүр дэлгэц үзүүлэх, эсвэл төхөөрөмжийн шугамыг хөтчийн
 * шугам гэж үзэх хоёрын аль нэг болно. Хоёулаа чимээгүй.
 */
describe("deviceLineFromHost", () => {
  it("хаягийн эхний шошгоор шугамыг таана", () => {
    expect(deviceLineFromHost("desktop.nexus.gerege.mn")?.line).toBe("desktop");
    expect(deviceLineFromHost("mobile.nexus.gerege.mn")?.line).toBe("mobile");
    expect(deviceLineFromHost("kiosk.nexus.gerege.mn")?.line).toBe("kiosk");
    expect(deviceLineFromHost("pos.nexus.gerege.mn")?.line).toBe("pos");
  });

  // Домэйн бүтнээр нь биш зөвхөн шошгоор тааруулдаг нь санаатай: staging,
  // preview, localhost гурав энэ файлыг хөндөхгүйгээр ажиллана.
  it("домэйны үлдсэн хэсгээс хамаарахгүй", () => {
    expect(deviceLineFromHost("mobile.nexus.staging.gerege.mn")?.line).toBe("mobile");
    expect(deviceLineFromHost("desktop.localhost:3000")?.line).toBe("desktop");
  });

  // 2026-09-02-нд шугам платформоор биш form factor-оор нэрлэгдэх болов.
  // nginx эдгээр нэрийг үйлчлэхээ больсон тул энд ч танигдах ёсгүй — танивал
  // байхгүй хост дээр ажиллах дүр эсгэсэн код үлдэнэ.
  it("хуучин платформын нэрсийг танихаа больсон", () => {
    for (const host of ["mac.nexus.gerege.mn", "win.nexus.gerege.mn", "ios.nexus.gerege.mn", "android.nexus.gerege.mn"]) {
      expect(deviceLineFromHost(host)).toBeNull();
    }
  });

  it("хөтчийн шугам ба утгагүй оролтод null", () => {
    expect(deviceLineFromHost("nexus.gerege.mn")).toBeNull();
    expect(deviceLineFromHost("")).toBeNull();
    expect(deviceLineFromHost(null)).toBeNull();
    expect(deviceLineFromHost(undefined)).toBeNull();
  });

  it("нүүр дэлгэцийн зам нь шугамын нэрээр гарна", () => {
    const line = deviceLineFromHost("desktop.nexus.gerege.mn");
    expect(line && lineHomePath(line)).toBe("/line/desktop");
  });
});
