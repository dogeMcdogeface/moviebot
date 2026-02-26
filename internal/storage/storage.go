package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

//
// -------------------- MODELS --------------------
//

type Movie struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Year    int             `json:"year"`

	AddedAt time.Time     `json:"added_at"` 
	Votes   map[string]bool `json:"votes"`
	Watched map[string]bool `json:"watched"`
	Poster  string          `json:"poster"`
}

type MessageRef struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int   `json:"message_id"`
}

//
// -------------------- STORE --------------------
//

type Store struct {
	moviesPath string
	indexPath  string

	saveDelay   time.Duration
	maxMessages int // max messages per movie/list

	mu       sync.RWMutex
	msgMu    sync.RWMutex
	movies   []Movie
	index    map[string][]MessageRef
	dirty    bool
	msgDirty bool

	saveTimer   *time.Timer
	msgSaveTimer *time.Timer
	timerMu     sync.Mutex
	msgTimerMu  sync.Mutex
}

//
// -------------------- INITIALIZATION --------------------
//

// NewStore creates a store and loads everything into memory.
func NewStore(moviesPath, indexPath string, saveDelay time.Duration, maxMessages int) *Store {
	s := &Store{
		moviesPath: moviesPath,
		indexPath:  indexPath,
		saveDelay:  saveDelay,
		maxMessages: maxMessages,
		index:      make(map[string][]MessageRef),
	}

	log.Printf("[STORE] Initializing store...")
	s.loadAll()
	log.Printf("[STORE] Initialization complete. Movies loaded: %d", len(s.movies))
	return s
}

func (s *Store) loadAll() {
	start := time.Now()

	// Load movies
	data, err := os.ReadFile(s.moviesPath)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &s.movies); err != nil {
			log.Printf("[STORE] Failed to parse movies: %v", err)
		}
	}

	// Load index
	idxData, err := os.ReadFile(s.indexPath)
	if err == nil && len(idxData) > 0 {
		if err := json.Unmarshal(idxData, &s.index); err != nil {
			log.Printf("[STORE] Failed to parse index: %v", err)
		}
	}

	log.Printf("[STORE] Loaded data from disk in %v", time.Since(start))
}

//
// -------------------- BULK SAVE LOGIC --------------------
//

func (s *Store) markDirty() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	s.dirty = true
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}

	s.saveTimer = time.AfterFunc(s.saveDelay, s.flushMovies)
}

func (s *Store) flushMovies() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return
	}

	start := time.Now()
	data, err := json.MarshalIndent(s.movies, "", "  ")
	if err != nil {
		log.Printf("[STORE] Failed to marshal movies: %v", err)
		return
	}

	if err := os.WriteFile(s.moviesPath, data, 0644); err != nil {
		log.Printf("[STORE] Failed to write movies: %v", err)
		return
	}

	s.dirty = false
	log.Printf("[STORE] Saved movies in %v", time.Since(start))
}

func (s *Store) markMsgDirty() {
	s.msgTimerMu.Lock()
	defer s.msgTimerMu.Unlock()

	s.msgDirty = true
	if s.msgSaveTimer != nil {
		s.msgSaveTimer.Stop()
	}

	s.msgSaveTimer = time.AfterFunc(s.saveDelay, s.flushMessages)
}

func (s *Store) flushMessages() {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	if !s.msgDirty {
		return
	}

	start := time.Now()
	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		log.Printf("[STORE] Failed to marshal message index: %v", err)
		return
	}

	if err := os.WriteFile(s.indexPath, data, 0644); err != nil {
		log.Printf("[STORE] Failed to write message index: %v", err)
		return
	}

	s.msgDirty = false
	log.Printf("[STORE] Saved message index in %v", time.Since(start))
}

//
// -------------------- MOVIE HELPERS --------------------
//

func generateMovieID(title string, year int) string {
	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("%s|%d", title, year)))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Store) NotifyNewMovie(title string, year int, poster string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range s.movies {
		if m.Title == title && m.Year == year {
			log.Printf("[STORE] Movie already exists: %s (%d)", title, year)
			return m.ID
		}
	}

	id := generateMovieID(title, year)
	m := Movie{
		ID:      id,
		Title:   title,
		Year:    year,
		AddedAt: time.Now(),
		Poster:  poster,
		Votes:   make(map[string]bool),
		Watched: make(map[string]bool),
	}

	s.movies = append(s.movies, m)
	log.Printf("[STORE] Added movie: %s (%d) [%s]", title, year, id)
	s.markDirty()
	return id
}

func (s *Store) GetMovieByID(id string) (Movie, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.movies {
		if m.ID == id {
			return m, true
		}
	}
	return Movie{}, false
}

func (s *Store) ToggleVoteByID(movieID, userID string) (Movie, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.movies {
		if s.movies[i].ID == movieID {
			if s.movies[i].Votes == nil {
				s.movies[i].Votes = make(map[string]bool)
			}
			if s.movies[i].Votes[userID] {
				delete(s.movies[i].Votes, userID)
				log.Printf("[STORE] User %s removed vote for %s", userID, s.movies[i].Title)
			} else {
				s.movies[i].Votes[userID] = true
				log.Printf("[STORE] User %s voted for %s", userID, s.movies[i].Title)
			}
			s.markDirty()
			return s.movies[i], nil
		}
	}
	return Movie{}, fmt.Errorf("movie not found")
}

func (s *Store) ToggleWatchedByID(movieID, userID string) (Movie, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.movies {
		if s.movies[i].ID == movieID {
			if s.movies[i].Watched == nil {
				s.movies[i].Watched = make(map[string]bool)
			}
			if s.movies[i].Watched[userID] {
				delete(s.movies[i].Watched, userID)
				log.Printf("[STORE] User %s marked %s as unwatched", userID, s.movies[i].Title)
			} else {
				s.movies[i].Watched[userID] = true
				log.Printf("[STORE] User %s marked %s as watched", userID, s.movies[i].Title)
			}
			s.markDirty()
			return s.movies[i], nil
		}
	}
	return Movie{}, fmt.Errorf("movie not found")
}

func (s *Store) GetAllMovies() []Movie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Movie(nil), s.movies...)
}

//
// -------------------- MESSAGE INDEX --------------------
//

// RegisterMessage adds a message ref for a movie or list.
// Keeps only last `maxMessages` messages per movie.
func (s *Store) RegisterMessage(movieID string, chatID int64, msgID int) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()

	msgs := append(s.index[movieID], MessageRef{ChatID: chatID, MessageID: msgID})
	if len(msgs) > s.maxMessages {
		msgs = msgs[len(msgs)-s.maxMessages:]
	}
	s.index[movieID] = msgs

	log.Printf("[STORE] Registered message %d for movie %s (total stored: %d)", msgID, movieID, len(msgs))

	s.markMsgDirty()
}

// GetMessages returns the last N messages for a movie/list.
func (s *Store) GetMessages(movieID string) []MessageRef {
	s.msgMu.RLock()
	defer s.msgMu.RUnlock()
	return append([]MessageRef(nil), s.index[movieID]...)
}

// GetAllMessages returns a copy of all stored message refs,
// keyed by movieID or special keys like "list".
func (s *Store) GetAllMessages() map[string][]MessageRef {
	s.msgMu.RLock()
	defer s.msgMu.RUnlock()

	out := make(map[string][]MessageRef, len(s.index))
	for key, refs := range s.index {
		out[key] = append([]MessageRef(nil), refs...)
	}

	return out
}