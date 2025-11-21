package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	ctx = context.Background()
	rdb *redis.Client

	// WebSocket Hub (Gerencia conexões com o Frontend)
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan DashboardData)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // Libera CORS para o Next.js
	}
	mutex = &sync.Mutex{}
)

// --- ESTRUTURAS DE DADOS ---
// Payload que será enviado para o Next.js
type DashboardData struct {
	HeroName       string `json:"hero_name"`
	ClockTime      int    `json:"clock_time"`
	ClockDisplay   string `json:"clock_display"`
	StrategyText   string `json:"strategy_text"`
	StrategyWarn   bool   `json:"strategy_warn"`
	HealthPercent  int    `json:"health_percent"`
	ManaPercent    int    `json:"mana_percent"`
	Gold           int    `json:"gold"`
	BuybackStatus  string `json:"buyback_status"` // "READY", "COOLDOWN", "NO_GOLD"
	BuybackMissing int    `json:"buyback_missing"`
	GPM            int    `json:"gpm"`
	KDA            string `json:"kda"`
	WandAlert      bool   `json:"wand_alert"`
}

// Estruturas do GSI
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
	Gold    int    `json:"gold"`
	GPM     int    `json:"gpm"`
	Kills   int    `json:"kills"`
	Deaths  int    `json:"deaths"`
	Assists int    `json:"assists"`
	Name    string `json:"name"`
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
	Slot0 Item `json:"slot0"`
	Slot1 Item `json:"slot1"`
	Slot2 Item `json:"slot2"`
	Slot3 Item `json:"slot3"`
	Slot4 Item `json:"slot4"`
	Slot5 Item `json:"slot5"`
}
type Item struct {
	Name    string `json:"name"`
	Charges int    `json:"charges"`
}

// --- ESTRATÉGIAS (Baseado no seu arquivo MD) ---
type Strategy struct {
	StartTime, EndTime int
	Message            string
	Warning            bool
}

var strategies = []Strategy{
	{0, 80, "💰 META: 675 Gold (Bottle) até 1:20", false},
	{100, 125, "💧 ALERTA: Runa de Água (Min 2)", true},
	{170, 190, "🟡 ALERTA: Bounty Rune (Min 3)", false},
	{230, 245, "💧 ALERTA: Runa de Água (Min 4)", true},
	{250, 300, "⚠️ TÁTICA: Stack Triângulo (Min 5)", true},
	{330, 360, "🗣️ COMANDO: Pedir Sup Mid -> Power Rune (6:00)", false},
	{390, 420, "🧠 ALERTA: Altar da Sabedoria (Min 7)", true},
	{460, 485, "⚔️ TÁTICA: Avançar wave -> Power Rune (8:00)", false},
	{530, 550, "🟡 ALERTA: Bounty Rune (Min 9)", false},
	{590, 610, "⚡ ALERTA: Power Rune (Min 10)", true},
}

// --- MAIN ---
func main() {
	rdb = redis.NewClient(&redis.Options{Addr: RedisAddr})

	fmt.Println("🚀 Dota 2 WebSocket Server Rodando!")
	fmt.Println("📡 GSI Input: :8080/ | WebSocket Output: :8080/ws")

	go startWorker()
	go handleMessages() // Gerencia envio para o frontend

	http.HandleFunc("/", handleIngest)      // Recebe do Dota
	http.HandleFunc("/ws", handleWebSocket) // Conecta com Next.js

	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

// --- HANDLERS HTTP/WS ---

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	// Mantém conexão viva
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
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
	defer r.Body.Close()
	rdb.LPush(ctx, RedisQueueKey, string(body))
	w.WriteHeader(http.StatusOK)
}

func handleMessages() {
	for data := range broadcast {
		mutex.Lock()
		for client := range clients {
			err := client.WriteJSON(data)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}

// --- WORKER ---

func startWorker() {
	fmt.Println("⚙️  Worker aguardando dados do Dota...")
	for {
		result, err := rdb.BRPop(ctx, 0, RedisQueueKey).Result()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		processLiveMatch(result[1])
	}
}

func processLiveMatch(jsonStr string) {
	var g LiveGameState
	if err := json.Unmarshal([]byte(jsonStr), &g); err != nil {
		return
	}
	if g.Map.GameState != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" {
		return
	}

	// Prepara os dados para o Dashboard
	clock := g.Map.ClockTime
	mins, secs := clock/60, clock%60
	if secs < 0 {
		secs *= -1
	}

	// 1. Estratégia Atual
	stratText := "Foco no Farm / Lane Control"
	stratWarn := false
	for _, s := range strategies {
		if clock >= s.StartTime && clock <= s.EndTime {
			stratText = s.Message
			stratWarn = s.Warning
			break
		}
	}

	// 2. Lógica de Buyback
	bbStatus := "READY"
	bbMissing := 0
	surplus := g.Player.Gold - g.Hero.BuybackCost
	if g.Hero.BuybackCooldown > 0 {
		bbStatus = "COOLDOWN"
	} else if surplus < 0 {
		bbStatus = "NO_GOLD"
		bbMissing = surplus * -1
	}

	// 3. Magic Wand Check
	wandAlert := false
	checkWand := func(i Item) {
		if (i.Name == "item_magic_wand" || i.Name == "item_magic_stick") && i.Charges >= 10 && g.Hero.HealthPercent < 40 {
			wandAlert = true
		}
	}
	checkWand(g.Items.Slot0)
	checkWand(g.Items.Slot1)
	checkWand(g.Items.Slot2)
	checkWand(g.Items.Slot3)
	checkWand(g.Items.Slot4)
	checkWand(g.Items.Slot5)

	// Monta o pacote
	data := DashboardData{
		HeroName:       g.Hero.Name,
		ClockTime:      clock,
		ClockDisplay:   fmt.Sprintf("%02d:%02d", mins, secs),
		StrategyText:   stratText,
		StrategyWarn:   stratWarn,
		HealthPercent:  g.Hero.HealthPercent,
		ManaPercent:    g.Hero.ManaPercent,
		Gold:           g.Player.Gold,
		BuybackStatus:  bbStatus,
		BuybackMissing: bbMissing,
		GPM:            g.Player.GPM,
		KDA:            fmt.Sprintf("%d/%d/%d", g.Player.Kills, g.Player.Deaths, g.Player.Assists),
		WandAlert:      wandAlert,
	}

	// Envia para o WebSocket
	broadcast <- data
}
