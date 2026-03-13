package auth

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
)

type SessionStore struct {
    redis *redis.Client
}

func NewSessionStore(redisClient *redis.Client) *SessionStore {
    return &SessionStore{redis: redisClient}
}

func sessionKey(sessionID string) string { return "session:" + sessionID }
func userSessionsKey(userID string) string { return "user_sessions:" + userID }

func (s *SessionStore) Save(ctx context.Context, sess Session, ttl time.Duration) error {
    raw, err := json.Marshal(sess)
    if err != nil {
        return err
    }
    pipe := s.redis.TxPipeline()
    pipe.Set(ctx, sessionKey(sess.SessionID), raw, ttl)
    pipe.SAdd(ctx, userSessionsKey(sess.UserID), sess.SessionID)
    pipe.Expire(ctx, userSessionsKey(sess.UserID), ttl)
    _, err = pipe.Exec(ctx)
    return err
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
    raw, err := s.redis.Get(ctx, sessionKey(sessionID)).Bytes()
    if err != nil {
        if err == redis.Nil {
            return nil, ErrSessionNotFound
        }
        return nil, err
    }
    var sess Session
    if err := json.Unmarshal(raw, &sess); err != nil {
        return nil, err
    }
    return &sess, nil
}

func (s *SessionStore) Delete(ctx context.Context, sessionID, userID string) error {
    pipe := s.redis.TxPipeline()
    pipe.Del(ctx, sessionKey(sessionID))
    if userID != "" {
        pipe.SRem(ctx, userSessionsKey(userID), sessionID)
    }
    _, err := pipe.Exec(ctx)
    return err
}

func (s *SessionStore) Touch(ctx context.Context, sess Session, ttl time.Duration) error {
    sess.LastSeenAt = time.Now().UTC()
    return s.Save(ctx, sess, ttl)
}
