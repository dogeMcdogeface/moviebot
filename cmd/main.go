package main

import (
	"os"
	"time"
	"log"
	"os/exec"
	
	"moviebot/internal/config"
	"moviebot/internal/omdb"
	"moviebot/internal/telegram"
    "moviebot/internal/storage"
	
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	baseDir    = "/config"
	configDir  = "/config/config"
	moviesFile = "/config/data/movies.json"
	messageIndexFile = "/config/data/message_index.json"
 	maxAlt          = 5
)


var BuildTime string // set at build via ldflags

func main() {
	log.Println("[BOT] Starting movie bot")
	log.Println("[BOT] Build time:", BuildTime)

	// Optional: self-restart watcher
	go watchSelf()

	/* =========================
	   LOAD CONFIG
	   ========================= */

	cfg, err := config.Load("/config/config")
	if err != nil {
		log.Fatal("[BOT] Failed to load config:", err)
	}

	/* =========================
	   INIT STORAGE
	   ========================= */

	store := storage.NewStore(
		cfg.Storage.MoviesFile,
		cfg.Storage.MessageIndexFile,
		cfg.Storage.SessionTTL,
		cfg.Storage.MaxMessages,
	)


	/* =========================
	   INIT OMDb
	   ========================= */

	omdbClient := omdb.NewClient(cfg.OmdbAPIKey)


	// Telegram bot
	tgBot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal("[Bot] Telegram init error:", err)
	}
	log.Printf("[Bot] Authorized on %s", tgBot.Self.UserName)

	bot := telegram.NewBot(tgBot, omdbClient, store, maxAlt)


	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tgBot.GetUpdatesChan(u)

	log.Println("[Bot] Listening for updates...")
	for update := range updates {
		bot.HandleUpdate(update)
	}
}


func watchSelf() {
    log.Println("[Watcher] Starting...")
    exePath, err := os.Executable()
    if err != nil {
        log.Println("[Watcher] Cannot get executable path:", err)
        return
    }

    info, err := os.Stat(exePath)
    if err != nil {
        log.Println("[Watcher] Cannot stat executable:", err)
        return
    }
    lastMod := info.ModTime()

    for {
        time.Sleep(2 * time.Second)
        info, err := os.Stat(exePath)
        if err != nil {
            log.Println("[Watcher] Cannot stat executable:", err)
            continue
        }
        if info.ModTime().After(lastMod) {
            log.Println("[Watcher] Executable changed, restarting...")
            // Re-exec self
            err := exec.Command(exePath, os.Args[1:]...).Start()
            if err != nil {
                log.Println("[Watcher] Failed to restart:", err)
            }
            os.Exit(0)
        }
    }
}