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
  buyback_status: "READY" | "COOLDOWN" | "NO_GOLD";
  buyback_missing: number;
  gpm: number;
  kda: string;
  wand_alert: boolean;
}

export default function DotaCoach() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    // Conecta no Go Backend
    const ws = new WebSocket("ws://localhost:8080/ws");

    ws.onopen = () => {
      console.log("Conectado ao Coach!");
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data);
        setData(parsed);
      } catch (e) {
        console.error("Erro JSON:", e);
      }
    };

    ws.onclose = () => setConnected(false);

    return () => ws.close();
  }, []);

  if (!connected || !data) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-900 text-white">
        <div className="text-center">
          <h1 className="text-2xl font-bold mb-2">Aguardando Conexão...</h1>
          <p className="text-slate-400">Inicie o Dota 2 e o servidor Go.</p>
        </div>
      </div>
    );
  }

  // Limpa o nome do herói (remove npc_dota_hero_)
  const heroName = data.hero_name.replace("npc_dota_hero_", "").toUpperCase().replace("_", " ");

  return (
    <div className="min-h-screen bg-slate-950 text-white font-sans p-6">
      {/* Top Bar: Tempo e KDA */}
      <header className="flex justify-between items-center mb-8 bg-slate-900 p-4 rounded-xl border border-slate-800 shadow-lg">
        <div className="text-xl font-bold text-slate-300">{heroName}</div>
        <div className="text-4xl font-mono font-black tracking-wider text-yellow-400">
          {data.clock_display}
        </div>
        <div className="text-right">
          <div className="text-sm text-slate-400">K/D/A</div>
          <div className="font-bold">{data.kda}</div>
        </div>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Coluna 1: Status Vitals */}
        <div className="space-y-6">
          {/* HP Bar */}
          <div className="bg-slate-900 p-4 rounded-xl border border-slate-800">
            <div className="flex justify-between mb-1">
              <span className="font-bold text-red-400">HEALTH</span>
              <span>{data.health_percent}%</span>
            </div>
            <div className="w-full bg-slate-800 h-6 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${
                  data.health_percent < 30 ? "bg-red-600 animate-pulse" : "bg-green-500"
                }`}
                style={{ width: `${data.health_percent}%` }}
              />
            </div>
          </div>

          {/* Mana Bar */}
          <div className="bg-slate-900 p-4 rounded-xl border border-slate-800">
            <div className="flex justify-between mb-1">
              <span className="font-bold text-blue-400">MANA</span>
              <span>{data.mana_percent}%</span>
            </div>
            <div className="w-full bg-slate-800 h-6 rounded-full overflow-hidden">
              <div
                className={`h-full bg-blue-500 transition-all duration-300 ${
                  data.mana_percent < 20 ? "bg-red-500" : ""
                }`}
                style={{ width: `${data.mana_percent}%` }}
              />
            </div>
          </div>

           {/* Wand Alert */}
           {data.wand_alert && (
            <div className="bg-yellow-500 text-black font-black text-center py-4 rounded-xl animate-bounce text-xl">
              ✨ USE SUA WAND! ✨
            </div>
          )}
        </div>

        {/* Coluna 2: Estratégia (Centro) */}
        <div className="md:col-span-1">
          <div className={`h-full flex flex-col items-center justify-center p-6 rounded-xl border-2 text-center shadow-2xl transition-colors duration-500 ${
            data.strategy_warn 
              ? "bg-yellow-900/30 border-yellow-500 text-yellow-200" 
              : "bg-slate-900 border-cyan-500/50 text-cyan-100"
          }`}>
            <div className="text-sm uppercase tracking-widest mb-4 opacity-70">Objetivo Atual</div>
            <div className="text-2xl font-bold leading-relaxed">
              {data.strategy_text}
            </div>
          </div>
        </div>

        {/* Coluna 3: Economia */}
        <div className="space-y-6">
          <div className="bg-slate-900 p-4 rounded-xl border border-slate-800">
             <div className="text-slate-400 text-sm">GPM</div>
             <div className="text-3xl font-bold text-yellow-500">{data.gpm}</div>
             <div className="text-slate-400 text-sm mt-2">Gold Atual</div>
             <div className="text-xl font-bold">{data.gold}</div>
          </div>

          {/* Buyback Status */}
          <div className={`p-4 rounded-xl border-2 transition-all ${
            data.buyback_status === "NO_GOLD" 
              ? "bg-red-900/50 border-red-500 animate-pulse" 
              : data.buyback_status === "COOLDOWN"
              ? "bg-slate-800 border-slate-600 opacity-50"
              : "bg-green-900/30 border-green-500"
          }`}>
            <div className="text-center">
              <div className="font-bold text-lg mb-1">BUYBACK STATUS</div>
              {data.buyback_status === "READY" && <div className="text-green-400 font-black text-2xl">PRONTO</div>}
              {data.buyback_status === "COOLDOWN" && <div className="text-slate-300 font-bold text-xl">EM RECARGA</div>}
              {data.buyback_status === "NO_GOLD" && (
                <div>
                  <div className="text-red-500 font-black text-2xl">SEM OURO</div>
                  <div className="text-red-300">Falta: {data.buyback_missing}</div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}