"use client";

import React, { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Sparkles, X, Send, Bot, User, Mic, Square, Volume2, Languages, Wrench } from "lucide-react";
import {
  Button,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Sheet,
  SheetClose,
  SheetContent,
  SheetTitle,
  SheetTrigger,
  cn,
} from "@gerege-systems/ui";

interface Message { role: "user" | "model"; text: string; tools?: string[]; degraded?: boolean }

const TARGETS = [
  ["mn", "Монгол"],
  ["en", "English"],
  ["ru", "Русский"],
  ["zh", "中文"],
  ["ko", "한국어"],
  ["ja", "日本語"],
] as const;

function toBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => { const r = new FileReader(); r.onerror=()=>reject(r.error); r.onload=()=>resolve(String(r.result).split(",")[1] || ""); r.readAsDataURL(blob); });
}
function play(audio: { mime: string; data: string }) { new Audio(`data:${audio.mime};base64,${audio.data}`).play().catch(() => undefined); }

/**
 * The copilot: a trigger for the header, and the drawer it opens.
 *
 * The drawer is a design-system Sheet, non-modal on purpose. It sits under the
 * header rather than over it, so the control that opened it stays visible and
 * can close it again, and it does not cover the page or take its pointer: this
 * is a copilot, and the work it is asked about has to stay readable — and
 * clickable — behind it. Escape still closes it, as it does every other layer
 * in this shell. The conversation is state of this component, not of the
 * sheet, so shutting the drawer does not lose it.
 *
 * `className` reaches the panel: the workarea's ribbon is shorter than the
 * browser header, and the panel starts where the chrome above it ends.
 */
export default function AICopilot({ className }: { className?: string }) {
  const { t, locale } = useI18n();
  const [open,setOpen]=useState(false), [prompt,setPrompt]=useState(""), [loading,setLoading]=useState(false);
  const [mode,setMode]=useState<"chat"|"translate">("chat"), [target,setTarget]=useState("en"), [recording,setRecording]=useState(false);
  const [messages,setMessages]=useState<Message[]>([{role:"model",text:t("ai.message.greeting")}]);
  const recorder=useRef<MediaRecorder|null>(null), chunks=useRef<Blob[]>([]), end=useRef<HTMLDivElement>(null);
  useEffect(()=>{end.current?.scrollIntoView({behavior:"smooth"})},[messages,loading]);

  async function dispatch(text?: string, audio?: {mime:string;data:string}) {
    const value=(text ?? prompt).trim(); if ((!value && !audio)||loading) return;
    const shown=value || t("ai.message.voice_message"); const prior=messages.filter(m=>!m.degraded).slice(-20).map(({role,text})=>({role,text}));
    setMessages(m=>[...m,{role:"user",text:shown}]); setPrompt(""); setLoading(true);
    try {
      if(mode==="translate") {
        const r=await api.translateAI({text:value||undefined,audio,target_lang:target});
        setMessages(m=>[...m,{role:"model",text:r.translated}]);
      } else {
        const r=await api.chatAI({prompt:value||undefined,audio,lang:locale,history:prior});
        setMessages(m=>[...m,{role:"model",text:r.reply||r.answer,tools:r.steps?.map(s=>s.tool),degraded:r.degraded}]);
      }
    } catch(e) { setMessages(m=>[...m,{role:"model",text:e instanceof Error?e.message:"AI request failed",degraded:true}]); }
    finally {setLoading(false)}
  }

  async function toggleRecord() {
    if(recording){recorder.current?.stop();return}
    try {
      const stream=await navigator.mediaDevices.getUserMedia({audio:true}); const rec=new MediaRecorder(stream); chunks.current=[];
      rec.ondataavailable=e=>{if(e.data.size)chunks.current.push(e.data)};
      rec.onstop=async()=>{setRecording(false);stream.getTracks().forEach(t=>t.stop());const blob=new Blob(chunks.current,{type:rec.mimeType});await dispatch(undefined,{mime:rec.mimeType.split(";")[0],data:await toBase64(blob)})};
      recorder.current=rec;rec.start();setRecording(true);setTimeout(()=>{if(rec.state==="recording")rec.stop()},30000);
    } catch {setMessages(m=>[...m,{role:"model",text:t("ai.message.microphone_denied"),degraded:true}])}
  }

  async function speak(text:string){try{play(await api.speakAI(text.slice(0,2000)))}catch{/* response remains readable */}}

  return (
    <Sheet open={open} onOpenChange={setOpen} modal={false}>
      <SheetTrigger asChild>
        <IconButton
          aria-label={t("ai.view.title")}
          title={t("ai.view.title")}
          icon={<Sparkles />}
          className="data-[state=open]:bg-accent-soft data-[state=open]:text-accent"
        />
      </SheetTrigger>
      <SheetContent
        side="right"
        showClose={false}
        aria-describedby={undefined}
        // Non-modal: a click on the page is work, not a request to close.
        onInteractOutside={(event) => event.preventDefault()}
        className={cn("top-14 h-auto w-[420px] max-w-full gap-0 p-0 max-lg:bottom-16", className)}
      >
        <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-line px-4">
          <div className="flex min-w-0 items-center gap-2">
            <Bot className="size-5 shrink-0 text-accent" aria-hidden />
            <div className="min-w-0">
              <SheetTitle className="truncate text-sm">{t("ai.view.subtitle")}</SheetTitle>
              <p className="truncate text-xs text-muted">{t("ai.view.engine_note")}</p>
            </div>
          </div>
          <SheetClose asChild>
            <IconButton aria-label={t("base.action.close")} icon={<X />} size="sm" />
          </SheetClose>
        </header>

        <div className="flex shrink-0 gap-1 border-b border-line bg-surface-2 p-1">
          {(["chat","translate"] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={mode===value}
              onClick={()=>setMode(value)}
              className={cn(
                "flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md px-3 text-xs font-medium outline-none",
                "transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                mode===value ? "bg-surface text-foreground shadow-sm" : "text-muted hover:text-foreground",
              )}
            >
              {value==="chat" ? <Bot className="size-4" aria-hidden /> : <Languages className="size-4" aria-hidden />}
              {value==="chat" ? t("ai.view.tab_chat") : t("ai.view.tab_translate")}
            </button>
          ))}
        </div>

        {mode==="translate" && (
          <div className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2 text-xs">
            <span className="text-muted">{t("ai.field.target_language")}</span>
            <Select value={target} onValueChange={setTarget}>
              <SelectTrigger size="sm" aria-label={t("ai.field.target_language")} className="w-36" />
              <SelectContent>
                {TARGETS.map(([code, label]) => <SelectItem key={code} value={code}>{label}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
        )}

        <div className="flex-1 space-y-4 overflow-y-auto p-4 text-xs" aria-live="polite">
          {messages.map((m,i) => (
            <div key={i} className={cn("flex gap-2", m.role==="user" ? "justify-end" : "justify-start")}>
              {m.role==="model" && <Bot className="mt-2 size-4 shrink-0 text-accent" aria-hidden />}
              <div className={cn("max-w-[85%] rounded-md p-3", m.role==="user" ? "bg-accent text-on-accent" : "bg-surface-2 text-foreground")}>
                <p className="whitespace-pre-wrap leading-relaxed">{m.text}</p>
                {m.role==="model" && !m.degraded && (
                  <IconButton aria-label={t("ai.action.listen")} title={t("ai.action.listen")} icon={<Volume2 />} size="sm" className="mt-1 text-accent" onClick={()=>void speak(m.text)} />
                )}
                {m.tools?.length ? <p className="mt-2 flex items-center gap-1 text-xs text-muted"><Wrench className="size-3" aria-hidden />{m.tools.join(", ")}</p> : null}
              </div>
              {m.role==="user" && <User className="mt-2 size-4 shrink-0 text-muted" aria-hidden />}
            </div>
          ))}
          {loading && <div className="animate-pulse text-muted">{t("ai.message.processing")}</div>}
          <div ref={end} />
        </div>

        <form onSubmit={e=>{e.preventDefault();void dispatch()}} className="flex shrink-0 items-center gap-2 border-t border-line bg-surface-2 p-3">
          <IconButton
            aria-label={t(recording ? "ai.action.stop_recording" : "ai.action.record")}
            aria-pressed={recording}
            icon={recording ? <Square /> : <Mic />}
            variant={recording ? "destructive" : "outline"}
            size="sm"
            disabled={loading}
            onClick={()=>void toggleRecord()}
          />
          <Input
            label={mode==="chat" ? t("ai.view.placeholder") : t("ai.view.translate_placeholder")}
            hideLabel
            size="sm"
            value={prompt}
            onChange={e=>setPrompt(e.target.value)}
            maxLength={4000}
            placeholder={mode==="chat" ? t("ai.view.placeholder") : t("ai.view.translate_placeholder")}
            className="flex-1"
          />
          <Button type="submit" size="sm" aria-label={t("ai.action.send")} disabled={loading||(!prompt.trim()&&!recording)} className="px-2.5">
            <Send aria-hidden />
          </Button>
        </form>
      </SheetContent>
    </Sheet>
  );
}
