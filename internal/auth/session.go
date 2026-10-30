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
	UpdateAt  time.Time `bson:"updateAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
	CreateIP  string    `bson:"createIp"`
	CreateUA  string    `bson:"createUa"`
}

// SessionInfo is one entry of the security page's active session list.
type SessionInfo struct {
	ID        string
	UserID    int64
	Kind      string
	CreatedAt time.Time
	UpdateAt  time.Time
	ExpiresAt time.Time
	IP        string
	UA        string
}

// ErrSessionNotFound is returned when a session digest does not belong to the
// caller.
var ErrSessionNotFound = errors.New("session not found")

const sessionTouchInterval = time.Minute

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
	if _, err := manager.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetName("session_ttl").SetExpireAfterSeconds(0),
	}); err != nil {
		return nil, err
	}
	if _, err := manager.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "kind", Value: 1}, {Key: "updateAt", Value: -1}},
		Options: options.Index().SetName("session_user_activity"),
	}); err != nil {
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

// IssueForUserMeta issues a browser session and records the client metadata the
// security page shows.
func (s *SessionManager) IssueForUserMeta(ctx context.Context, uid int64, ua, ip string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := s.now().UTC()
	session := Session{
		UserID:    uid,
		Kind:      "user",
		ExpiresAt: now.Add(s.ttl),
	}
	if s.collection == nil {
		s.mu.Lock()
		s.sessions[token] = session
		s.mu.Unlock()
		return token, nil
	}
	if _, err := s.collection.InsertOne(ctx, sessionDocument{
		ID:        hashToken(token),
		UserID:    uid,
		Kind:      "user",
		CreatedAt: now,
		UpdateAt:  now,
		ExpiresAt: session.ExpiresAt,
		CreateIP:  ip,
		CreateUA:  ua,
	}); err != nil {
		return "", err
	}
	return token, nil
}

// Digest is the identifier the security page uses instead of the raw token.
func (s *SessionManager) Digest(token string) string {
	return hashToken(token)
}

// ListByUser returns the browser sessions of one user, newest activity first.
func (s *SessionManager) ListByUser(ctx context.Context, uid int64) ([]SessionInfo, error) {
	if s.collection == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		result := make([]SessionInfo, 0, len(s.sessions))
		for token, session := range s.sessions {
			if session.UserID != uid || session.Kind != "user" {
				continue
			}
			result = append(result, SessionInfo{
				ID: hashToken(token), UserID: session.UserID, Kind: session.Kind,
				CreatedAt: session.ExpiresAt.Add(-s.ttl), UpdateAt: session.ExpiresAt.Add(-s.ttl),
				ExpiresAt: session.ExpiresAt,
			})
		}
		return result, nil
	}
	cursor, err := s.collection.Find(ctx, bson.M{"userId": uid, "kind": "user"},
		options.Find().SetSort(bson.D{{Key: "updateAt", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := make([]SessionInfo, 0)
	for cursor.Next(ctx) {
		var document sessionDocument
		if err := cursor.Decode(&document); err != nil {
			return nil, err
		}
		result = append(result, SessionInfo{
			ID: document.ID, UserID: document.UserID, Kind: document.Kind,
			CreatedAt: document.CreatedAt, UpdateAt: document.UpdateAt, ExpiresAt: document.ExpiresAt,
			IP: document.CreateIP, UA: document.CreateUA,
		})
	}
	return result, cursor.Err()
}

// LastActiveAt returns the most recent browser-session activity for a user.
// Judge sessions are ignored because they do not represent a signed-in person.
func (s *SessionManager) LastActiveAt(ctx context.Context, uid int64) (time.Time, bool, error) {
	if uid <= 0 || s.collection == nil {
		return time.Time{}, false, nil
	}
	var document struct {
		UpdateAt time.Time `bson:"updateAt"`
	}
	err := s.collection.FindOne(ctx, bson.M{"userId": uid, "kind": "user"},
		options.FindOne().
			SetProjection(bson.M{"updateAt": 1}).
			SetSort(bson.D{{Key: "updateAt", Value: -1}})).
		Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if document.UpdateAt.IsZero() {
		return time.Time{}, false, nil
	}
	return document.UpdateAt, true, nil
}

// RevokeDigest deletes one session of the user identified by its digest.
func (s *SessionManager) RevokeDigest(ctx context.Context, uid int64, digest string) error {
	if digest == "" {
		return ErrSessionNotFound
	}
	if s.collection == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for token, session := range s.sessions {
			if hashToken(token) == digest && session.UserID == uid {
				delete(s.sessions, token)
				return nil
			}
		}
		return ErrSessionNotFound
	}
	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": digest, "userId": uid})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeUser deletes every session of one user, used after a password change.
func (s *SessionManager) RevokeUser(ctx context.Context, uid int64) error {
	if s.collection == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for token, session := range s.sessions {
			if session.UserID == uid {
				delete(s.sessions, token)
			}
		}
		return nil
	}
	_, err := s.collection.DeleteMany(ctx, bson.M{"userId": uid})
	return err
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
	// Keep "last seen" roughly current without writing on every request. One
	// minute keeps the profile online indicator inside Hydro's five-minute
	// freshness window.
	if now := s.now().UTC(); now.Sub(document.UpdateAt) > sessionTouchInterval {
		_, _ = s.collection.UpdateOne(ctx, bson.M{"_id": hash}, bson.M{"$set": bson.M{"updateAt": now}})
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
