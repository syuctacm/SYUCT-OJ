package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Session struct {
	UserID    int64
	Kind      string
	ExpiresAt time.Time
}

type sessionDocument struct {
	ID        string    `bson:"_id"`
	UserID    int64     `bson:"userId"`
	Kind      string    `bson:"kind"`
	CreatedAt time.Time `bson:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[string]Session
	collection *mongo.Collection
	ttl        time.Duration
	now        func() time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{
		sessions: make(map[string]Session),
		ttl:      ttl,
		now:      time.Now,
	}
}

func NewMongoSessionManager(ctx context.Context, db *mongo.Database, ttl time.Duration) (*SessionManager, error) {
	manager := NewSessionManager(ttl)
	manager.sessions = nil
	manager.collection = db.Collection("session")
	_, err := manager.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetName("session_ttl").SetExpireAfterSeconds(0),
	})
	if err != nil {
		return nil, err
	}
	return manager, nil
}

func (s *SessionManager) TTL() time.Duration {
	return s.ttl
}

// Issue creates a judge-compatible session without a browser user identity.
func (s *SessionManager) Issue() (string, error) {
	return s.IssueContext(context.Background())
}

func (s *SessionManager) IssueContext(ctx context.Context) (string, error) {
	return s.IssueForContext(ctx, 0, "judge")
}

func (s *SessionManager) IssueForUser(uid int64) (string, error) {
	return s.IssueForUserContext(context.Background(), uid)
}

func (s *SessionManager) IssueForUserContext(ctx context.Context, uid int64) (string, error) {
	return s.IssueForContext(ctx, uid, "user")
}

func (s *SessionManager) IssueFor(uid int64, kind string) (string, error) {
	return s.IssueForContext(context.Background(), uid, kind)
}

func (s *SessionManager) IssueForContext(ctx context.Context, uid int64, kind string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := s.now().UTC()
	session := Session{
		UserID:    uid,
		Kind:      kind,
		ExpiresAt: now.Add(s.ttl),
	}
	if s.collection == nil {
		s.mu.Lock()
		s.sessions[token] = session
		s.mu.Unlock()
		return token, nil
	}
	_, err := s.collection.InsertOne(ctx, sessionDocument{
		ID:        hashToken(token),
		UserID:    session.UserID,
		Kind:      session.Kind,
		CreatedAt: now,
		ExpiresAt: session.ExpiresAt,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionManager) Get(token string) (Session, bool) {
	return s.GetContext(context.Background(), token)
}

func (s *SessionManager) GetContext(ctx context.Context, token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	if s.collection == nil {
		s.mu.RLock()
		session, ok := s.sessions[token]
		s.mu.RUnlock()
		if !ok {
			return Session{}, false
		}
		if s.now().After(session.ExpiresAt) {
			s.mu.Lock()
			delete(s.sessions, token)
			s.mu.Unlock()
			return Session{}, false
		}
		return session, true
	}

	hash := hashToken(token)
	var document sessionDocument
	if err := s.collection.FindOne(ctx, bson.M{"_id": hash}).Decode(&document); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			slog.Warn("unable to load session", "error", err)
		}
		return Session{}, false
	}
	if s.now().After(document.ExpiresAt) {
		_, _ = s.collection.DeleteOne(ctx, bson.M{"_id": hash})
		return Session{}, false
	}
	return Session{UserID: document.UserID, Kind: document.Kind, ExpiresAt: document.ExpiresAt}, true
}

func (s *SessionManager) Validate(token string) bool {
	return s.ValidateContext(context.Background(), token)
}

func (s *SessionManager) ValidateContext(ctx context.Context, token string) bool {
	_, ok := s.GetContext(ctx, token)
	return ok
}

func (s *SessionManager) Revoke(token string) {
	s.RevokeContext(context.Background(), token)
}

func (s *SessionManager) RevokeContext(ctx context.Context, token string) {
	if token == "" {
		return
	}
	if s.collection == nil {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return
	}
	if _, err := s.collection.DeleteOne(ctx, bson.M{"_id": hashToken(token)}); err != nil {
		slog.Warn("unable to revoke session", "error", err)
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
