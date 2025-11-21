"use client";

import { useEffect, useState } from "react";

interface DashboardData {
  hero_name: string;
  clock_display: string;
  strategy_text: string;
  strategy_warn: boolean;
  health_percent: number;
  mana_percent: number;
  gold: number;
  last_hits: number;
  denies: number;
  buyback_status: "READY" | "COOLDOWN" | "NO_GOLD";
  buyback_missing: number;
  gpm: number;
  kda: string;
  wand_alert: boolean;
  tp_alert: boolean;
  hp_regen_alert: boolean;   // NOVO
  mana_regen_alert: boolean; // NOVO
}

export default function DotaCoach() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [status, setStatus] = useState<"DISCONNECTED" | "CONNECTED" | "HAS_DATA">("DISCONNECTED");

  useEffect(() => {
    const ws = new WebSocket("ws://localhost:8080/ws");

    ws.onopen = () => {
      console.log("WS Conectado");
      setStatus("CONNECTED");
    };

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data);
        setData(parsed);
        setStatus("HAS_DATA");
      } catch (e) {
        console.error("Erro Parse:", e);
      }
    };

    ws.onclose = () => setStatus("DISCONNECTED");
    return () => ws.close();
  }, []);

  if (status === "DISCONNECTED") {
    return (
      <div className="flex h-screen items-center justify-center bg-red-950 text-white">
        <div className="text-center animate-pulse">
          <h1 className="text-3xl font-bold mb-2">🔴 Desconectado</h1>
          <p>Verifique se o 'go run main.go' está rodando.</p>
        </div>
      </div>
    );
  }

  if (status === "CONNECTED" && !data) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-900 text-white">
        <div className="text-center">
          <h1 className="text-3xl font-bold mb-4 text-yellow-400">🟡 Aguardando Partida...</h1>
          <p className="text-slate-400 mb-2">Conexão com Coach: OK</p>
          <p className="text-sm opacity-50">Entre em uma partida para iniciar.</p>
        </div>
      </div>
    );
  }

  if (!data) return null; 

  const heroName = data.hero_name.replace("npc_dota_hero_", "").toUpperCase().replace("_", " ");

  return (
    <div className="min-h-screen bg-slate-950 text-white font-sans p-6">
      {/* Header */}
      <header className="flex justify-between items-center mb-8 bg-slate-900 p-4 rounded-xl border border-slate-800 shadow-lg">
        <div className="flex flex-col w-1/4">
            <span className="text-xs text-slate-500 uppercase font-bold">Herói</span>
            <span className="text-xl font-black text-white tracking-wide truncate">{heroName}</span>
        </div>
        
        <div className="text-5xl font-mono font-black tracking-widest text-yellow-400 drop-shadow-lg text-center w-2/4">
          {data.clock_display}
        </div>
        
        <div className="flex justify-end gap-6 w-1/4 text-right">
          <div>
            <div className="text-xs text-slate-500 uppercase font-bold">CS (LH/DN)</div>
            <div className="text-xl font-bold text-green-400">
              {data.last_hits} <span className="text-slate-600">/</span> <span className="text-red-400">{data.denies}</span>
            </div>
          </div>
          <div>
            <div className="text-xs text-slate-500 uppercase font-bold">K / D / A</div>
            <div className="text-xl font-bold">{data.kda}</div>
          </div>
        </div>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Coluna 1: Vitais e Alertas de Item */}
        <div className="space-y-6">
          {/* HP Bar */}
          <div className="bg-slate-900 p-5 rounded-xl border border-slate-800">
            <div className="flex justify-between mb-2">
              <span className="font-bold text-red-400">HEALTH</span>
              <span className="font-mono">{data.health_percent}%</span>
            </div>
            <div className="w-full bg-slate-800 h-4 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${
                  data.health_percent < 30 ? "bg-red-600 animate-pulse shadow-[0_0_15px_rgba(220,38,38,0.7)]" : "bg-gradient-to-r from-green-600 to-green-400"
                }`}
                style={{ width: `${data.health_percent}%` }}
              />
            </div>
          </div>

          {/* Mana Bar */}
          <div className="bg-slate-900 p-5 rounded-xl border border-slate-800">
            <div className="flex justify-between mb-2">
              <span className="font-bold text-blue-400">MANA</span>
              <span className="font-mono">{data.mana_percent}%</span>
            </div>
            <div className="w-full bg-slate-800 h-4 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${
                  data.mana_percent < 20 ? "bg-red-500" : "bg-gradient-to-r from-blue-600 to-blue-400"
                }`}
                style={{ width: `${data.mana_percent}%` }}
              />
            </div>
          </div>

           {/* ALERTAS DE EMERGÊNCIA */}
           <div className="space-y-4">
             {/* Alerta de Regen HP */}
             {data.hp_regen_alert && (
              <div className="bg-orange-600 text-white font-bold text-center py-3 rounded-xl animate-pulse border-2 border-orange-400">
                💊 COMPRE HP REGEN!
                <div className="text-xs font-normal mt-1">Balsamo, Tango ou Bottle</div>
              </div>
             )}

             {/* Alerta de Regen Mana */}
             {data.mana_regen_alert && (
              <div className="bg-blue-700 text-white font-bold text-center py-3 rounded-xl animate-pulse border-2 border-blue-400">
                💧 COMPRE MANA REGEN!
                <div className="text-xs font-normal mt-1">Clarity, Mango ou Bottle</div>
              </div>
             )}

             {/* Alerta de Wand */}
             {data.wand_alert && (
              <div className="bg-yellow-400 text-black font-black text-center py-4 rounded-xl animate-[bounce_1s_infinite] text-xl border-4 border-yellow-600 shadow-xl">
                ⚡ USE SUA WAND AGORA! ⚡
              </div>
             )}
             
             {/* Alerta de TP */}
             {data.tp_alert && (
              <div className="bg-red-600 text-white font-black text-center py-4 rounded-xl animate-pulse text-xl border-4 border-red-800 shadow-[0_0_20px_rgba(220,38,38,0.8)]">
                🚫 VOCÊ ESTÁ SEM TP! 🚫
                <div className="text-sm font-normal mt-1">Compre um Scroll ou viaje para base.</div>
              </div>
             )}
           </div>
        </div>

        {/* Coluna 2: Estratégia */}
        <div className="md:col-span-1">
          <div className={`h-full flex flex-col items-center justify-center p-6 rounded-xl border-2 text-center shadow-2xl transition-all duration-500 ${
            data.strategy_warn 
              ? "bg-yellow-950/40 border-yellow-500 text-yellow-200 scale-105" 
              : "bg-slate-900 border-cyan-500/30 text-cyan-100"
          }`}>
            <div className="text-xs uppercase tracking-[0.2em] mb-4 opacity-60">Objetivo Atual</div>
            <div className="text-2xl font-bold leading-tight drop-shadow-md">
              {data.strategy_text}
            </div>
          </div>
        </div>

        {/* Coluna 3: Economia */}
        <div className="space-y-6">
          <div className="bg-slate-900 p-5 rounded-xl border border-slate-800 grid grid-cols-2 gap-4">
             <div>
                 <div className="text-slate-500 text-xs font-bold uppercase">GPM</div>
                 <div className="text-3xl font-bold text-yellow-500">{data.gpm}</div>
             </div>
             <div className="text-right">
                 <div className="text-slate-500 text-xs font-bold uppercase">Gold</div>
                 <div className="text-3xl font-bold text-white">{data.gold}</div>
             </div>
          </div>

          <div className={`p-5 rounded-xl border-2 transition-all ${
            data.buyback_status === "NO_GOLD" 
              ? "bg-red-950/80 border-red-500 animate-pulse shadow-red-900/50 shadow-lg" 
              : data.buyback_status === "COOLDOWN"
              ? "bg-slate-800 border-slate-600 opacity-60 grayscale"
              : "bg-green-950/40 border-green-500"
          }`}>
            <div className="text-center">
              <div className="font-bold text-sm mb-2 tracking-widest uppercase opacity-80">Buyback Status</div>
              {data.buyback_status === "READY" && <div className="text-green-400 font-black text-3xl tracking-wider">DISPONÍVEL</div>}
              {data.buyback_status === "COOLDOWN" && <div className="text-slate-300 font-bold text-2xl">EM RECARGA</div>}
              {data.buyback_status === "NO_GOLD" && (
                <div>
                  <div className="text-red-500 font-black text-3xl mb-1">SEM OURO</div>
                  <div className="text-red-300 font-mono text-lg">Falta: {data.buyback_missing}g</div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}