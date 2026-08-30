"use client";

import React, { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Sparkles, X, Send, Bot, User, Mic, Square, Volume2, Languages, Wrench } from "lucide-react";

interface Message { role: "user" | "model"; text: string; tools?: string[]; degraded?: boolean }

function toBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => { const r = new FileReader(); r.onerror=()=>reject(r.error); r.onload=()=>resolve(String(r.result).split(",")[1] || ""); r.readAsDataURL(blob); });
}
function play(audio: { mime: string; data: string }) { new Audio(`data:${audio.mime};base64,${audio.data}`).play().catch(() => undefined); }

export default function AICopilot() {
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

  // Escape closes it, as it does every other layer in this shell. A drawer that
  // can only be dismissed by finding its own close button is one people leave
  // open and then work around.
  useEffect(()=>{
    if(!open) return;
    const onKey=(e:KeyboardEvent)=>{if(e.key==="Escape")setOpen(false)};
    window.addEventListener("keydown",onKey);
    return ()=>window.removeEventListener("keydown",onKey);
  },[open]);

  return <>
    <button onClick={()=>setOpen(v=>!v)} aria-label={t("ai.view.title")} title={t("ai.view.title")} aria-expanded={open} aria-controls="gerege-copilot"
      className={`gerege-copilot-trigger w-10 h-10 rounded-full grid place-items-center border transition shrink-0 ${open?"border-white/40 bg-white/25 text-white":"border-white/25 bg-white/10 text-white hover:bg-white/20"}`}><Sparkles className="w-5 h-5"/></button>
    {/* Always mounted, moved rather than removed: a conversation that
        disappears when the drawer is shut is one nobody trusts to leave, and a
        panel that is only rendered while open cannot animate out. */}
    <section id="gerege-copilot" aria-hidden={!open} aria-label={t("ai.view.subtitle")}
      className={`gerege-copilot-drawer bg-white border-s border-slate-200 shadow-2xl flex flex-col ${open?"is-open":""}`}>
      <header className="bg-gradient-to-r from-slate-900 to-indigo-950 p-4 text-white flex items-center justify-between shrink-0"><div className="flex items-center gap-2"><Bot className="w-5 h-5 text-indigo-300"/><div><h3 className="font-bold text-sm text-white">{t("ai.view.subtitle")}</h3><p className="text-[11px] text-slate-400">{t("ai.view.engine_note")}</p></div></div><button onClick={()=>setOpen(false)}><X className="w-5 h-5"/></button></header>
      <div className="flex border-b bg-slate-50 p-1 gap-1">{(["chat","translate"] as const).map(v=><button key={v} onClick={()=>setMode(v)} className={`flex-1 rounded-lg px-3 py-2 text-xs font-semibold ${mode===v?"bg-white text-indigo-700 shadow-sm":"text-slate-500"}`}>{v==="chat"?<Bot className="inline w-4 h-4 mr-1"/>:<Languages className="inline w-4 h-4 mr-1"/>}{v==="chat"?t("ai.view.tab_chat"):t("ai.view.tab_translate")}</button>)}</div>
      {mode==="translate"&&<div className="px-3 py-2 border-b text-xs flex items-center gap-2"><span>{t("ai.field.target_language")}</span><select value={target} onChange={e=>setTarget(e.target.value)} className="border rounded px-2 py-1"><option value="mn">Монгол</option><option value="en">English</option><option value="ru">Русский</option><option value="zh">中文</option><option value="ko">한국어</option><option value="ja">日本語</option></select></div>}
      <div className="flex-1 p-4 overflow-y-auto space-y-4 text-xs" aria-live="polite">{messages.map((m,i)=><div key={i} className={`flex gap-2 ${m.role==="user"?"justify-end":"justify-start"}`}>{m.role==="model"&&<Bot className="w-4 h-4 mt-2 text-indigo-500 shrink-0"/>}<div className={`max-w-[85%] rounded-xl p-3 ${m.role==="user"?"bg-indigo-600 text-white":"bg-slate-100 text-slate-800 border"}`}><p className="leading-relaxed whitespace-pre-wrap">{m.text}</p>{m.role==="model"&&!m.degraded&&<button onClick={()=>speak(m.text)} className="mt-2 text-indigo-600" title={t("ai.action.listen")}><Volume2 className="w-4 h-4"/></button>}{m.tools?.length?<p className="mt-2 text-[10px] text-slate-500 flex gap-1"><Wrench className="w-3 h-3"/>{m.tools.join(", ")}</p>:null}</div>{m.role==="user"&&<User className="w-4 h-4 mt-2 text-slate-400 shrink-0"/>}</div>)}{loading&&<div className="text-slate-500 animate-pulse">{t("ai.message.processing")}</div>}<div ref={end}/></div>
      <form onSubmit={e=>{e.preventDefault();void dispatch()}} className="p-3 border-t bg-slate-50 flex items-center gap-2"><button type="button" onClick={()=>void toggleRecord()} disabled={loading} className={`p-2 rounded-lg ${recording?"bg-red-600 text-white":"border bg-white text-slate-600"}`}>{recording?<Square className="w-4 h-4"/>:<Mic className="w-4 h-4"/>}</button><input value={prompt} onChange={e=>setPrompt(e.target.value)} maxLength={4000} placeholder={mode==="chat"?t("ai.view.placeholder"):t("ai.view.translate_placeholder")} className="flex-1 px-3 py-2 text-xs border rounded-lg focus:ring-2 focus:ring-indigo-500 outline-none"/><button disabled={loading||(!prompt.trim()&&!recording)} className="bg-indigo-600 text-white p-2 rounded-lg disabled:opacity-50"><Send className="w-4 h-4"/></button></form>
    </section>
  </>;
}
