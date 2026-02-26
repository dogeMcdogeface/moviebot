package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"moviebot/internal/omdb"
	"moviebot/internal/storage"
)

type Bot struct {
	API    *tgbotapi.BotAPI
	OMDb   *omdb.OMDbClient
	Store  *storage.Store
	MaxAlt int

	sessMu   sync.Mutex
	sessions map[string]*userSession // sessionID -> session
}

type userSession struct {
	ID            string
	UserID        int64
	ChatID        int64
	Query         string
	Results       []omdb.SearchResult
	OrigMessageID int
	ActiveMsgIDs  []int
	
	WaitingForQuery bool
	PromptMessageID int
}

// =====================================================
// INIT
// =====================================================

func NewBot(api *tgbotapi.BotAPI, omdb *omdb.OMDbClient, store *storage.Store, maxAlt int) *Bot {
	return &Bot{
		API:      api,
		OMDb:     omdb,
		Store:    store,
		MaxAlt:   maxAlt,
		sessions: make(map[string]*userSession),
	}
}

// =====================================================
// UPDATE HANDLER
// =====================================================

func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		b.handleCallback(update.CallbackQuery)
	}
	if update.Message != nil && update.Message.IsCommand() {
		b.handleCommand(update.Message)
	}
	if update.Message != nil && !update.Message.IsCommand() {
		b.handleText(update.Message)
	}
}

func (b *Bot) handleText(msg *tgbotapi.Message) {
	// MUST match how you created it
	sessionID := fmt.Sprintf("wait:%d:%d", msg.Chat.ID, msg.From.ID)

	b.sessMu.Lock()
	sess, ok := b.sessions[sessionID]
	b.sessMu.Unlock()

	if !ok || !sess.WaitingForQuery {
		return
	}

	// 🔥 Ensure this is a reply to our forced prompt
	if msg.ReplyToMessage == nil ||
		msg.ReplyToMessage.MessageID != sess.PromptMessageID {
		return
	}

	query := strings.TrimSpace(msg.Text)
	if query == "" {
		return
	}

	// Remove waiting session
	b.cleanupSession(sessionID)

	log.Printf("[OMDb] Searching for '%s' requested by %s", query, msg.From.UserName)

	results, err := b.OMDb.Search(query)
	if err != nil || len(results) == 0 {
		b.API.Send(tgbotapi.NewMessage(msg.Chat.ID, "No results found"))
		return
	}

	// Create real movie-selection session
	newSessionID := fmt.Sprintf("%d:%d", msg.From.ID, time.Now().UnixNano())

	newSess := &userSession{
		ID:            newSessionID,
		UserID:        msg.From.ID,
		ChatID:        msg.Chat.ID,
		Query:         query,
		Results:       results,
		OrigMessageID: msg.MessageID, // reply to user’s answer
	}

	b.sessMu.Lock()
	b.sessions[newSessionID] = newSess
	b.sessMu.Unlock()

	b.sendMovieSelection(newSess, 0)
}
// =====================================================
// COMMANDS
// =====================================================

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {

	case "start":
		log.Printf("[BOT] /start from %s", msg.From.UserName)
		b.sendKeyboard(msg.Chat.ID)

	case "movie":
		query := strings.TrimSpace(msg.CommandArguments())

if query == "" {
	// Create chat-scoped waiting session (safer for groups)
	sessionID := fmt.Sprintf("wait:%d:%d", msg.Chat.ID, msg.From.ID)

	waitSess := &userSession{
		ID:              sessionID,
		UserID:          msg.From.ID,
		ChatID:          msg.Chat.ID,
		WaitingForQuery: true,

	}

	// Send forced reply prompt
	prompt := tgbotapi.NewMessage(
		msg.Chat.ID,
		"🎬 What movie would you like to search for?",
	)

	prompt.ReplyToMessageID = msg.MessageID

	prompt.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true, // only the command sender sees forced reply UI
	}

	sent, err := b.API.Send(prompt)
	if err != nil {
		return
	}

	// Store prompt message ID so we can validate the reply
	waitSess.PromptMessageID = sent.MessageID

	b.sessMu.Lock()
	b.sessions[sessionID] = waitSess
	b.sessMu.Unlock()

	return
}

		log.Printf("[OMDb] Searching for '%s' requested by %s", query, msg.From.UserName)
		results, err := b.OMDb.Search(query)
		if err != nil || len(results) == 0 {
			b.API.Send(tgbotapi.NewMessage(msg.Chat.ID, "No results found"))
			return
		}

		sessionID := fmt.Sprintf("%d:%d", msg.From.ID, time.Now().UnixNano())

		sess := &userSession{
			ID:            sessionID,
			UserID:        msg.From.ID,
			ChatID:        msg.Chat.ID,
			Query:         query,
			Results:       results,
			OrigMessageID: msg.MessageID,
		}

		b.sessMu.Lock()
		b.sessions[sessionID] = sess
		b.sessMu.Unlock()

		b.sendMovieSelection(sess, 0)

case "list":
	args := strings.TrimSpace(msg.CommandArguments())

	if args != "" {
		if format, ok := tableFormats[args]; ok {
			// ✅ Valid format selected
			currentTableFormat = format
			log.Printf("[BOT] Table format set to %s", args)

		} else {
			// ❌ Invalid format
			log.Printf("[BOT] Invalid table format '%s' requested by %s", args, msg.From.UserName)

			// Build keyboard with available formats
			var row []tgbotapi.KeyboardButton
			for key := range tableFormats {
				row = append(row, tgbotapi.NewKeyboardButton("/list "+key))
			}

			keyboard := tgbotapi.NewReplyKeyboard(row) // single row of buttons
			keyboard.ResizeKeyboard = true
			keyboard.OneTimeKeyboard = true

			msgToSend := tgbotapi.NewMessage(
				msg.Chat.ID,
				"Unknown table format. Please choose one of the available formats:",
			)
			msgToSend.ReplyMarkup = keyboard
			b.API.Send(msgToSend)
			return
		}
	}

	log.Printf("[BOT] /list from %s", msg.From.UserName)
	b.sendList(msg.Chat.ID, msg.MessageID)
	}
}

// =====================================================
// CALLBACKS
// =====================================================

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	if cb == nil || cb.From == nil {
		return
	}

	data := cb.Data
	userID := cb.From.ID
	userIDStr := strconv.FormatInt(userID, 10)

	log.Printf("[CALLBACK] '%s' from %s", data, cb.From.UserName)

	// -------------------------
	// GLOBAL CALLBACKS
	// -------------------------

	if strings.HasPrefix(data, "vote|") {
		id := strings.TrimPrefix(data, "vote|")
		movie, err := b.Store.ToggleVoteByID(id, userIDStr)
		if err == nil {
			b.syncMovie(movie)
		}
		return
	}

	if strings.HasPrefix(data, "watched|") {
		id := strings.TrimPrefix(data, "watched|")
		movie, err := b.Store.ToggleWatchedByID(id, userIDStr)
		if err == nil {
			b.syncMovie(movie)
		}
		return
	}

	// -------------------------
	// SESSION CALLBACKS
	// format: action|sessionID|index
	// -------------------------

	parts := strings.Split(data, "|")
	if len(parts) != 3 {
		log.Printf("[CALLBACK] Malformed data: %s", data)
		return
	}

	action, sessionID, idxStr := parts[0], parts[1], parts[2]
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		return
	}

	b.sessMu.Lock()
	sess, ok := b.sessions[sessionID]
	b.sessMu.Unlock()

	if !ok || sess == nil {
		log.Printf("[CALLBACK] Session not found: %s", sessionID)

		// Remove inline keyboard so old buttons are dead
		if cb.Message != nil {
			log.Printf("[CALLBACK] Removing buttons from stale message %d", cb.Message.MessageID)
			b.removeInlineKeyboard(cb.Message.Chat.ID, cb.Message.MessageID)
		}

		// Send a toast to the user
		b.answerToast(cb, "⏱️ Sorry, this message is too old")

		return
	}

	if sess.UserID != userID {
		log.Printf("[CALLBACK] User %d tried to access session %s", userID, sessionID)
		b.answerToast(cb, "🚫 This movie selection isn’t for you")
		return
	}

	if index < 0 || index >= len(sess.Results) {
		return
	}

	if cb.Message != nil {
		b.API.Request(tgbotapi.NewDeleteMessage(cb.Message.Chat.ID, cb.Message.MessageID))
	}

	switch action {

	case "select":
		m := sess.Results[index]
		year, _ := strconv.Atoi(m.Year)
		log.Printf("[BOT] %s selected '%s' (%d)", cb.From.UserName, m.Title, year)

		movieID := b.Store.NotifyNewMovie(m.Title, year, m.Poster)
		if movieID != "" {
			b.createOrUpdateVoteMessage(sess.ChatID, movieID)
		}

		b.cleanupSession(sessionID)

	case "alt":
		b.sendMovieSelection(sess, index)
	}
}

// =====================================================
// SESSION HELPERS
// =====================================================

func (b *Bot) cleanupSession(sessionID string) {
	b.sessMu.Lock()
	defer b.sessMu.Unlock()

	sess, ok := b.sessions[sessionID]
	if !ok {
		return
	}

	for _, msgID := range sess.ActiveMsgIDs {
		b.API.Request(tgbotapi.NewDeleteMessage(sess.ChatID, msgID))
	}

	delete(b.sessions, sessionID)
}

// =====================================================
// MOVIE SELECTION
// =====================================================

func (b *Bot) sendMovieSelection(sess *userSession, offset int) {
if offset >= len(sess.Results) || offset >= b.MaxAlt {

    // Clean up previous selection messages
    for _, msgID := range sess.ActiveMsgIDs {
        b.API.Request(tgbotapi.NewDeleteMessage(sess.ChatID, msgID))
    }

    // Notify user
    msg := tgbotapi.NewMessage(sess.ChatID, "❌ No more alternatives available.")
    msg.ReplyToMessageID = sess.OrigMessageID
    b.API.Send(msg)

    // Destroy session
    b.cleanupSession(sess.ID)
    return
}

	for _, msgID := range sess.ActiveMsgIDs {
		b.API.Request(tgbotapi.NewDeleteMessage(sess.ChatID, msgID))
	}
	sess.ActiveMsgIDs = nil

	m := sess.Results[offset]
	text := fmt.Sprintf(
		"*%s* (%s)\n\n[Poster](%s)",
		m.Title, m.Year, m.Poster,
	)

	msg := tgbotapi.NewMessage(sess.ChatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = sess.OrigMessageID
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"✅ Select this movie",
				fmt.Sprintf("select|%s|%d", sess.ID, offset),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				"👎 Search Another",
				fmt.Sprintf("alt|%s|%d", sess.ID, offset+1),
			),
		),
	)

	sent, err := b.API.Send(msg)
	if err != nil {
		return
	}

	sess.ActiveMsgIDs = append(sess.ActiveMsgIDs, sent.MessageID)

	go func(chatID int64, msgID int, sessionID string) {
		time.Sleep(5 * time.Minute)
		b.API.Request(tgbotapi.NewDeleteMessage(chatID, msgID))
		b.cleanupSession(sessionID)
	}(sent.Chat.ID, sent.MessageID, sess.ID)
}


func (b *Bot) answerToast(cb *tgbotapi.CallbackQuery, text string) {
    resp := tgbotapi.NewCallback(cb.ID, text)
    resp.ShowAlert = false // toast, not popup
    b.API.Send(resp)
}

func (b *Bot) removeInlineKeyboard(chatID int64, messageID int) error {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, tgbotapi.InlineKeyboardMarkup{})
	edit.ReplyMarkup = nil // THIS removes the keyboard

	_, err := b.API.Send(edit)
	return err
}

// =====================================================
// VOTES / LIST (UNCHANGED LOGIC)
// =====================================================

func (b *Bot) buildVoteMessageConfig(movie storage.Movie) (string, tgbotapi.InlineKeyboardMarkup) {
	text := fmt.Sprintf(
		"*%s* (%d)\n\n👍 Votes: *%d*\n👁 Watched: %d\n\n[Poster](%s)\n\nVote 👍 to add to the list or mark as watched.",
		movie.Title, movie.Year, len(movie.Votes), len(movie.Watched), movie.Poster,
	)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("👍 Vote (%d)", len(movie.Votes)),
				fmt.Sprintf("vote|%s", movie.ID),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("👁️ Watched (%d)", len(movie.Watched)),
				fmt.Sprintf("watched|%s", movie.ID),
			),
		),
	)
	return text, keyboard
}

func (b *Bot) createOrUpdateVoteMessage(chatID int64, movieID string) {
	movie, exists := b.Store.GetMovieByID(movieID)
	if !exists {
		return
	}

	text, keyboard := b.buildVoteMessageConfig(movie)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	sent, err := b.API.Send(msg)
	if err != nil {
		return
	}

	b.Store.RegisterMessage(movie.ID, sent.Chat.ID, sent.MessageID)
}

func (b *Bot) syncMovie(movie storage.Movie) {
	text, keyboard := b.buildVoteMessageConfig(movie)
	refs := b.Store.GetMessages(movie.ID)

	for _, ref := range refs {
		editText := tgbotapi.NewEditMessageText(ref.ChatID, ref.MessageID, text)
		editText.ParseMode = "Markdown"
		b.API.Send(editText)

		editKeyboard := tgbotapi.NewEditMessageReplyMarkup(ref.ChatID, ref.MessageID, keyboard)
		b.API.Send(editKeyboard)
	}

	b.syncListMessages()
}


func (b *Bot) sendList(chatID int64, replyTo int) {
    text := "```\n" +storage.BuildListMessage( b.Store.GetAllMovies(), currentTableFormat) + "\n```" // Use the new list builder logic
   
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = replyTo
	sent, _ := b.API.Send(msg)
	b.Store.RegisterMessage("list", sent.Chat.ID, sent.MessageID)
}

func (b *Bot) syncListMessages() {
    text :=  "```\n" + storage.BuildListMessage( b.Store.GetAllMovies(), currentTableFormat)+ "\n```" // Use the new list builder logic

	refs := b.Store.GetMessages("list")
	for _, ref := range refs {
		edit := tgbotapi.NewEditMessageText(ref.ChatID, ref.MessageID, text)
		edit.ParseMode = "Markdown"
		b.API.Send(edit)
	}
}

// =====================================================
// LIST BUILDER
// =====================================================

var tableFormats = map[string]storage.TableFormat{
"default": {
    Columns: []storage.MovieColumn{
        {"Title", 	25, storage.FormatTitle},
        {"Year",	4,	storage.FormatYear},
        {"Votes", 	5,  storage.FormatVotes},
        {"Seen", 	4,  storage.FormatWatched},
    },
    SortBy:         	storage.SortByVotes,   	// Default sort by votes
    SeparateWatched: 	true,         			// Default to separate watched/unwatched movies
},
	"detail": {    Columns: []storage.MovieColumn{
        {"Title", 	20, storage.FormatTitle},
        {"Year", 	4,  storage.FormatYear},
        {"Votes", 	5,  storage.FormatVotes},
        {"Seen", 	4,  storage.FormatWatched},
        {"Added", 	10, storage.FormatAdded}, 	
    },
    SortBy:         	storage.SortByVotes,   	// Default sort by votes
    SeparateWatched: 	true,         			// Default to separate watched/unwatched movies
},
	"wide": {
    Columns: []storage.MovieColumn{
        {"Title", 	40, storage.FormatTitle},
        {"Year", 	4,  storage.FormatYear},
        {"Votes", 	5,  storage.FormatVotes},
        {"Seen", 	4,  storage.FormatWatched},
        {"Added", 	10, storage.FormatAdded}, 	
    },
    SortBy:         	storage.SortByVotes,   	// Default sort by votes
    SeparateWatched: 	true,         			// Default to separate watched/unwatched movies
},
}
var currentTableFormat = tableFormats["default"]




// =====================================================
// UI
// =====================================================

func (b *Bot) sendKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "👍 Movie bot ready")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/movie"),
			tgbotapi.NewKeyboardButton("/list"),
		),
	)
	b.API.Send(msg)
}