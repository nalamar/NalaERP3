package auth

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

// LoginRateLimiter sperrt eine IP-Adresse temporaer, nachdem sie innerhalb
// eines Zeitfensters zu viele FEHLGESCHLAGENE Anmeldeversuche erzeugt hat.
// Erfolgreiche Anmeldungen zaehlen bewusst nicht mit (siehe RegisterFailure/
// Reset) - das schuetzt vor Brute-Force/Credential-Stuffing, ohne normale
// Nutzer bei wiederholten korrekten Logins (z.B. mehrere Browser-Tabs,
// Integrationstests) zu blockieren.
type LoginRateLimiter struct {
	redis       *redis.Client
	maxAttempts int
	window      time.Duration
}

func NewLoginRateLimiter(redisClient *redis.Client, maxAttempts int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{redis: redisClient, maxAttempts: maxAttempts, window: window}
}

func loginRateLimitKey(ipAddress string) string {
	host, _, err := net.SplitHostPort(ipAddress)
	if err != nil {
		host = ipAddress
	}
	return "login_rl:" + host
}

// Blocked meldet, ob die IP-Adresse aktuell gesperrt ist. Ein nil-Limiter
// oder maxAttempts<=0 deaktiviert die Pruefung (kein Blockieren).
func (l *LoginRateLimiter) Blocked(ctx context.Context, ipAddress string) (bool, error) {
	if l == nil || l.redis == nil || l.maxAttempts <= 0 {
		return false, nil
	}
	count, err := l.redis.Get(ctx, loginRateLimitKey(ipAddress)).Int()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return count >= l.maxAttempts, nil
}

// RegisterFailure zaehlt einen fehlgeschlagenen Anmeldeversuch fuer die
// IP-Adresse und startet bei der ersten Zaehlung das Zeitfenster.
func (l *LoginRateLimiter) RegisterFailure(ctx context.Context, ipAddress string) error {
	if l == nil || l.redis == nil || l.maxAttempts <= 0 {
		return nil
	}
	key := loginRateLimitKey(ipAddress)
	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		if err := l.redis.Expire(ctx, key, l.window).Err(); err != nil {
			return err
		}
	}
	return nil
}

// Reset loescht den Fehlversuch-Zaehler nach einer erfolgreichen Anmeldung.
func (l *LoginRateLimiter) Reset(ctx context.Context, ipAddress string) error {
	if l == nil || l.redis == nil {
		return nil
	}
	return l.redis.Del(ctx, loginRateLimitKey(ipAddress)).Err()
}
