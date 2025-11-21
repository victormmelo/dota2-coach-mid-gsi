package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// --- CONFIGURAÇÕES ---
const (
	RedisAddr     = "localhost:6380"
	RedisQueueKey = "dota_live_queue"
	ServerPort    = ":8080"
)

var (
	ctx       = context.Background()
	rdb       *redis.Client
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan DashboardData)
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mutex     = &sync.Mutex{}
)

// --- DADOS ENVIADOS AO FRONTEND ---
type DashboardData struct {
	HeroName       string `json:"hero_name"`
	ClockTime      int    `json:"clock_time"`
	ClockDisplay   string `json:"clock_display"`
	StrategyText   string `json:"strategy_text"`
	StrategyWarn   bool   `json:"strategy_warn"`
	HealthPercent  int    `json:"health_percent"`
	ManaPercent    int    `json:"mana_percent"`
	Gold           int    `json:"gold"`
	LastHits       int    `json:"last_hits"`
	Denies         int    `json:"denies"`
	BuybackStatus  string `json:"buyback_status"`
	BuybackMissing int    `json:"buyback_missing"`
	GPM            int    `json:"gpm"`
	KDA            string `json:"kda"`
	WandAlert      bool   `json:"wand_alert"`
	TpAlert        bool   `json:"tp_alert"`
	HpRegenAlert   bool   `json:"hp_regen_alert"`   // NOVO
	ManaRegenAlert bool   `json:"mana_regen_alert"` // NOVO
}

// --- ESTRUTURAS DO GSI (Dota) ---
type LiveGameState struct {
	Map    Map    `json:"map"`
	Player Player `json:"player"`
	Hero   Hero   `json:"hero"`
	Items  Items  `json:"items"`
}
type Map struct {
	ClockTime int    `json:"clock_time"`
	GameState string `json:"game_state"`
}
type Player struct {
	Gold     int    `json:"gold"`
	GPM      int    `json:"gpm"`
	Kills    int    `json:"kills"`
	Deaths   int    `json:"deaths"`
	Assists  int    `json:"assists"`
	LastHits int    `json:"last_hits"`
	Denies   int    `json:"denies"`
	Name     string `json:"name"`
}
type Hero struct {
	Name            string `json:"name"`
	Level           int    `json:"level"`
	Alive           bool   `json:"alive"`
	Health          int    `json:"health"`
	MaxHealth       int    `json:"max_health"`
	HealthPercent   int    `json:"health_percent"`
	Mana            int    `json:"mana"`
	MaxMana         int    `json:"max_mana"`
	ManaPercent     int    `json:"mana_percent"`
	BuybackCost     int    `json:"buyback_cost"`
	BuybackCooldown int    `json:"buyback_cooldown"`
}
type Items struct {
	Slot0    Item `json:"slot0"`
	Slot1    Item `json:"slot1"`
	Slot2    Item `json:"slot2"`
	Slot3    Item `json:"slot3"`
	Slot4    Item `json:"slot4"`
	Slot5    Item `json:"slot5"`
	Teleport Item `json:"teleport0"`
}
type Item struct {
	Name    string `json:"name"`
	Charges int    `json:"charges"`
}

// --- ESTRATÉGIAS ---
type Strategy struct {
	StartTime, EndTime int
	Message            string
	Warning            bool
}

var strategies = []Strategy{
	// --- EARLY GAME ---
	{100, 125, "💧 ALERTA: Runa de Água (Min 2)", true},
	{165, 195, "🟡 IMPORTANTE: Bounty Rune + Watcher (Min 3)", true},
	{230, 245, "💧 ALERTA: Runa de Água (Min 4)", true},
	{250, 300, "⚠️ TÁTICA: Stack Triângulo (Min 5)", true},
	{310, 360, "🔥 ALL-IN: Matar Mid (c/ Sup) -> PUSH (Catapulta) -> Runa (6:00)", true},
	{390, 450, "🧠 ALERTA: Altar da Sabedoria (Min 7) - Não perca XP!", true},
	{450, 490, "⚡ TÁTICA: Preparar Runa de Poder (8:00) + Watcher", false},
	{530, 550, "🟡 ALERTA: Bounty Rune (Min 9)", false},
	{590, 630, "⚡ ALERTA: Power Rune (Min 10) + Catapulta", true},

	// --- MID GAME TRANSITION (10-20 min) ---

	// Min 10-12: Defesa de Torre
	{630, 720, "🛡️ MACRO: Defenda T1 Mid! Empurre waves e rotacione para Safe Lane.", false},

	// Min 14: Wisdom Rune (CRÍTICO)
	// Começa avisar 13:30 (810s) até 14:15 (855s)
	{810, 855, "🧠 CRÍTICO: Wisdom Rune (Min 14)! Roube a do inimigo se puder.", true},

	// Min 15: Bounty
	{880, 915, "🟡 ALERTA: Bounty Rune (Min 15)", false},

	// Min 16-19: Power Spike / Lótus
	{960, 1140, "⚔️ DECISÃO: Tem item Core (Blink/BKB)? SIM: Smoke/Luta | NÃO: Farma!", false},

	// Min 20: Tormentor (Shard)
	// Avisa 19:30 (1170s) até 20:30 (1230s)
	{1170, 1230, "💎 OBJETIVO: Tormentor (Min 20) = Shard Grátis! Chame o time.", true},
}

// --- MAIN ---
func main() {
	rdb = redis.NewClient(&redis.Options{Addr: RedisAddr})
	fmt.Println("🚀 Dota 2 Smart Coach Rodando...")
	go startWorker()
	go handleMessages()
	http.HandleFunc("/", handleIngest)
	http.HandleFunc("/ws", handleWebSocket)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			break
		}
	}
}

func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	body, _ := io.ReadAll(r.Body)
	rdb.LPush(ctx, RedisQueueKey, string(body))
	w.WriteHeader(http.StatusOK)
}

func handleMessages() {
	for data := range broadcast {
		mutex.Lock()
		for client := range clients {
			client.WriteJSON(data)
		}
		mutex.Unlock()
	}
}

func startWorker() {
	for {
		result, err := rdb.BRPop(ctx, 0, RedisQueueKey).Result()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		processLiveMatch(result[1])
	}
}

// Helper para verificar se existe algum item da lista no inventário
func hasAnyItem(items Items, targetNames []string) bool {
	slots := []Item{items.Slot0, items.Slot1, items.Slot2, items.Slot3, items.Slot4, items.Slot5}
	for _, slot := range slots {
		for _, target := range targetNames {
			if strings.Contains(slot.Name, target) {
				return true
			}
		}
	}
	return false
}

func processLiveMatch(jsonStr string) {
	var g LiveGameState
	if err := json.Unmarshal([]byte(jsonStr), &g); err != nil {
		return
	}
	state := g.Map.GameState
	if state == "DOTA_GAMERULES_STATE_INIT" || state == "DOTA_GAMERULES_STATE_WAIT_FOR_PLAYERS_TO_LOAD" {
		return
	}

	clock := g.Map.ClockTime
	mins, secs := clock/60, clock%60
	if secs < 0 {
		secs *= -1
	}

	// --- ESTRATÉGIA ---
	stratText := "Foco no Farm / Lane Control"
	stratWarn := false

	if clock >= 0 && clock <= 80 {
		targetGold, targetLH := 675, 6
		goldMissing := targetGold - g.Player.Gold
		lhMissing := targetLH - g.Player.LastHits
		if goldMissing <= 0 {
			stratText = "✅ BOTTLE GARANTIDA! Compre agora!"
			stratWarn = true
		} else {
			if lhMissing < 0 {
				lhMissing = 0
			}
			stratText = fmt.Sprintf("💰 META BOTTLE: Falta %d Gold (Aprox. %d LHs)", goldMissing, lhMissing)
		}
	} else if state == "DOTA_GAMERULES_STATE_PRE_GAME" {
		stratText = "⏳ PREPARAÇÃO: Cheque seus itens e posição para runas!"
		stratWarn = true
	} else {
		for _, s := range strategies {
			if clock >= s.StartTime && clock <= s.EndTime {
				stratText = s.Message
				stratWarn = s.Warning
				break
			}
		}
	}

	// --- ALERTAS DE REGENERAÇÃO (NOVO) ---
	hpRegenAlert := false
	manaRegenAlert := false

	// Regra HP: Antes do min 5 (300s), precisa de Bottle, Flask (Balsamo) ou Tango
	if clock < 300 && clock > 0 {
		// item_flask = Healing Salve
		if !hasAnyItem(g.Items, []string{"item_bottle", "item_flask", "item_tango"}) {
			hpRegenAlert = true
		}
	}

	// Regra Mana: Entre min 2 (120s) e min 5 (300s), precisa de Bottle, Mango ou Clarity
	if clock >= 120 && clock < 300 {
		// item_enchanted_mango = Mango
		if !hasAnyItem(g.Items, []string{"item_bottle", "item_enchanted_mango", "item_clarity"}) {
			manaRegenAlert = true
		}
	}

	// --- BUYBACK ---
	bbStatus := "READY"
	bbMissing := 0
	if g.Hero.BuybackCost > 0 {
		surplus := g.Player.Gold - g.Hero.BuybackCost
		if g.Hero.BuybackCooldown > 0 {
			bbStatus = "COOLDOWN"
		} else if surplus < 0 {
			bbStatus = "NO_GOLD"
			bbMissing = surplus * -1
		}
	}

	// --- ALERTA DE WAND ---
	wandAlert := false
	checkWand := func(i Item) {
		if (strings.Contains(i.Name, "magic_wand") || strings.Contains(i.Name, "magic_stick")) && i.Charges >= 10 && g.Hero.HealthPercent < 40 {
			wandAlert = true
		}
	}
	checkWand(g.Items.Slot0)
	checkWand(g.Items.Slot1)
	checkWand(g.Items.Slot2)
	checkWand(g.Items.Slot3)
	checkWand(g.Items.Slot4)
	checkWand(g.Items.Slot5)

	// --- ALERTA DE TP ---
	tpAlert := false
	if clock > 0 {
		hasTP := false
		if g.Items.Teleport.Name == "item_tpscroll" {
			hasTP = true
		}
		if !hasTP {
			if hasAnyItem(g.Items, []string{"item_travel_boots"}) {
				hasTP = true
			}
		}
		if !hasTP {
			tpAlert = true
		}
	}

	heroName := g.Hero.Name
	if heroName == "" {
		heroName = "---"
	}

	data := DashboardData{
		HeroName:       heroName,
		ClockTime:      clock,
		ClockDisplay:   fmt.Sprintf("%02d:%02d", mins, secs),
		StrategyText:   stratText,
		StrategyWarn:   stratWarn,
		HealthPercent:  g.Hero.HealthPercent,
		ManaPercent:    g.Hero.ManaPercent,
		Gold:           g.Player.Gold,
		LastHits:       g.Player.LastHits,
		Denies:         g.Player.Denies,
		BuybackStatus:  bbStatus,
		BuybackMissing: bbMissing,
		GPM:            g.Player.GPM,
		KDA:            fmt.Sprintf("%d/%d/%d", g.Player.Kills, g.Player.Deaths, g.Player.Assists),
		WandAlert:      wandAlert,
		TpAlert:        tpAlert,
		HpRegenAlert:   hpRegenAlert,   // NOVO
		ManaRegenAlert: manaRegenAlert, // NOVO
	}

	broadcast <- data
}
