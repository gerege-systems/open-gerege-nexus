"use client";

import ReportsPage from "@/app/reports/page";

/**
 * Тайлан — documents аппын дотор. Дэлгэц нь платформын тайлангийн нэгдсэн
 * ажиллуулагч бөгөөд энд зөвхөн энэ аппын тайлангуудаар шүүгдэнэ: гэрээний
 * бүртгэл, гарын үсэг хүлээгдэж буй, гарын үсгийн бүртгэл, гэрээний урсгал,
 * дуусах гэрээ. Параметр, хүснэгт, график, Excel экспорт — бүгд нэгдсэн
 * ажиллуулагчийнх: тайлан нэмэхэд энэ файл өөрчлөгдөхгүй.
 */
export default function DocumentsReportsPage() {
  return <ReportsPage appFilter="io.gerege.nexus.documents" />;
}
