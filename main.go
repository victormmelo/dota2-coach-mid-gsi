package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// --- CONFIGURAÇÕES ---
const (
	RedisAddr     = "localhost:6380" // Sua porta configurada no Docker
	RedisQueueKey = "dota_live_queue"
	ServerPort    = ":8080"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

// --- ESTRUTURAS DE DADOS (Baseadas no seu JSON de Jogador) ---

type LiveGameState struct {
	Provider Provider `json:"provider"`
	Map      Map      `json:"map"`
	Player   Player   `json:"player"`
	Hero     Hero     `json:"hero"`
	Items    Items    `json:"items"`
}

type Provider struct {
	Timestamp int `json:"timestamp"`
}

type Map struct {
	ClockTime int    `json:"clock_time"`
	GameState string `json:"game_state"`
	Paused    bool   `json:"paused"`
	MatchID   string `json:"matchid"`
}

type Player struct {
	Gold           int    `json:"gold"`
	GoldReliable   int    `json:"gold_reliable"`
	GoldUnreliable int    `json:"gold_unreliable"`
	GPM            int    `json:"gpm"`
	XPM            int    `json:"xpm"`
	Name           string `json:"name"`
	Activity       string `json:"activity"` // "playing", "dead"
	Kills          int    `json:"kills"`
	Deaths         int    `json:"deaths"`
	Assists        int    `json:"assists"`
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
	RespawnSeconds  int    `json:"respawn_seconds"`
}

// Mapeamento direto dos itens que estão na raiz do objeto "items"
type Items struct {
	Slot0    Item `json:"slot0"`
	Slot1    Item `json:"slot1"`
	Slot2    Item `json:"slot2"`
	Slot3    Item `json:"slot3"`
	Slot4    Item `json:"slot4"`
	Slot5    Item `json:"slot5"`
	Teleport Item `json:"teleport0"`
	Neutral  Item `json:"neutral0"`
}

type Item struct {
	Name     string `json:"name"`
	CanCast  bool   `json:"can_cast"`
	Cooldown int    `json:"cooldown"`
	Charges  int    `json:"charges"` // Importante para Wand/Stick
	Passive  bool   `json:"passive"`
}

// --- FUNÇÃO PRINCIPAL ---

func main() {
	// 1. Conectar ao Redis
	rdb = redis.NewClient(&redis.Options{
		Addr: RedisAddr,
	})

	// Teste de conexão
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no Redis (%s): %v", RedisAddr, err)
	}

	fmt.Println("🚀 Dota 2 Live Coach Iniciado!")
	fmt.Println("📡 Ouvindo na porta 8080 e processando via Redis...")

	// 2. Iniciar o Worker (Consumidor) em paralelo
	go startWorker()

	// 3. Iniciar o Servidor HTTP (Produtor)
	http.HandleFunc("/", handleIngest)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

// --- PRODUTOR (HTTP -> REDIS) ---
func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	defer r.Body.Close()

	// Joga na fila do Redis imediatamente (LPUSH)
	// Usamos LPush para inserir na cabeça da lista, mas idealmente
	// se quisermos processar na ordem, o worker fará RPOP (ou BRPop no final)
	// Aqui vamos jogar e deixar o worker pegar.
	rdb.LPush(ctx, RedisQueueKey, string(body))

	w.WriteHeader(http.StatusOK)
}

// --- CONSUMIDOR (WORKER -> DASHBOARD) ---
func startWorker() {
	fmt.Println("⚙️  Worker de Análise rodando...")

	for {
		// BRPop bloqueia até chegar um dado na fila (Timeout 0 = infinito)
		result, err := rdb.BRPop(ctx, 0, RedisQueueKey).Result()
		if err != nil {
			// Se der erro de conexão, espera um pouco e tenta de novo
			time.Sleep(1 * time.Second)
			continue
		}

		// result[1] contém o JSON
		jsonStr := result[1]
		processLiveMatch(jsonStr)
	}
}

// --- LÓGICA DO COACH ---

func processLiveMatch(jsonStr string) {
	var g LiveGameState
	if err := json.Unmarshal([]byte(jsonStr), &g); err != nil {
		return
	}

	// Filtro: Só mostra dashboard se o jogo estiver rolando
	if g.Map.GameState != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" {
		if g.Map.GameState == "DOTA_GAMERULES_STATE_PRE_GAME" {
			fmt.Print("\033[H\033[2J") // Limpa tela
			fmt.Println("⏳ FASE DE PREPARAÇÃO")
			fmt.Println("-> Verifique seus itens iniciais.")
			fmt.Println("-> Planeje a disputa da Runa de Água/Bounty.")
		}
		return
	}

	// Limpa o terminal para efeito de atualização em tempo real
	// (Funciona no WSL/Linux/Mac)
	fmt.Print("\033[H\033[2J")

	// Formatação de tempo
	clock := g.Map.ClockTime
	minutes := clock / 60
	seconds := clock % 60
	if seconds < 0 {
		seconds *= -1
	} // Correção para tempo negativo (antes do 0:00)

	// --- CABEÇALHO ---
	fmt.Println("========================================")
	fmt.Printf("  DOTA 2 LIVE COACH  |  %02d:%02d\n", minutes, seconds)
	fmt.Println("========================================")

	fmt.Printf("HERÓI: %-15s LVL: %d\n", cleanName(g.Hero.Name), g.Hero.Level)
	fmt.Printf("K/D/A: %d/%d/%d          GPM: %d\n", g.Player.Kills, g.Player.Deaths, g.Player.Assists, g.Player.GPM)
	fmt.Println("----------------------------------------")

	// --- 1. MONITOR DE VITAIS (HP/MANA) ---
	printVitals(g)

	// --- 2. CALCULADORA DE BUYBACK (CRÍTICO) ---
	printBuybackStatus(g)

	// --- 3. ALERTA DE ITENS (WAND) ---
	checkEmergencyItems(g)

	fmt.Println("\n----------------------------------------")
	fmt.Println(" (Ctrl+C para encerrar o Coach)")
}

func printVitals(g LiveGameState) {
	// Cores ANSI
	red := "\033[31m"
	green := "\033[32m"
	yellow := "\033[33m"
	blue := "\033[36m"
	reset := "\033[0m"

	// Lógica HP
	hpColor := green
	if g.Hero.HealthPercent < 30 {
		hpColor = red // Crítico
	} else if g.Hero.HealthPercent < 60 {
		hpColor = yellow // Atenção
	}

	// Lógica Mana
	manaColor := blue
	if g.Hero.ManaPercent < 20 {
		manaColor = red // Sem mana
	}

	status := "VIVO"
	if !g.Hero.Alive {
		status = fmt.Sprintf("%sMORTO (%ds)%s", red, g.Hero.RespawnSeconds, reset)
	}

	fmt.Printf("STATUS: %s\n", status)
	fmt.Printf("HP:   %s%3d%% (%d/%d)%s\n", hpColor, g.Hero.HealthPercent, g.Hero.Health, g.Hero.MaxHealth, reset)
	fmt.Printf("MANA: %s%3d%% (%d/%d)%s\n", manaColor, g.Hero.ManaPercent, g.Hero.Mana, g.Hero.MaxMana, reset)
}

func printBuybackStatus(g LiveGameState) {
	// Só calculamos buyback se o jogo tiver passado de 10 min ou se o custo for relevante
	if g.Map.ClockTime < 600 {
		return
	}

	fmt.Println("\n--- ECONOMIA & BUYBACK ---")

	cost := g.Hero.BuybackCost
	currentGold := g.Player.Gold
	surplus := currentGold - cost

	// Cores
	red := "\033[31m"   // Perigo
	green := "\033[32m" // Seguro
	bgRed := "\033[41m" // Fundo Vermelho (Alerta Máximo)
	reset := "\033[0m"

	fmt.Printf("Ouro Atual: %d | Custo BB: %d\n", currentGold, cost)

	if g.Hero.BuybackCooldown > 0 {
		fmt.Printf("Estado: %sEM RECARGA (%ds)%s\n", red, g.Hero.BuybackCooldown, reset)
	} else if surplus >= 0 {
		fmt.Printf("Estado: %sDISPONÍVEL (+%d gold)%s\n", green, surplus, reset)
	} else {
		// Alerta visual forte
		fmt.Printf("%s⚠️  SEM BUYBACK! FALTA %d DE OURO ⚠️%s\n", bgRed, surplus*-1, reset)
	}
}

func checkEmergencyItems(g LiveGameState) {
	// Verifica se tem Magic Wand/Stick com muitas cargas e pouca vida
	checkWand := func(i Item) {
		isWand := (i.Name == "item_magic_wand" || i.Name == "item_magic_stick")
		if isWand && i.Charges >= 10 && g.Hero.HealthPercent < 40 && g.Hero.Alive {
			fmt.Printf("\n✨ \033[33mUSE SUA WAND! (%d Cargas)\033[0m ✨\n", i.Charges)
		}
	}

	// Verifica slots principais
	checkWand(g.Items.Slot0)
	checkWand(g.Items.Slot1)
	checkWand(g.Items.Slot2)
	checkWand(g.Items.Slot3)
	checkWand(g.Items.Slot4)
	checkWand(g.Items.Slot5)
}

// Função auxiliar para limpar nomes (ex: "npc_dota_hero_axe" -> "AXE")
func cleanName(raw string) string {
	if len(raw) > 14 {
		return raw[14:] // Remove "npc_dota_hero_"
	}
	return raw
}
