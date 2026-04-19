package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const ctxSessionKey = "helios_session"

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		sess, ok := GetSession(cookie)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		TouchSession(sess.ID)

		// Slide the cookie expiry so browsers don't drop it mid-session.
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(SessionCookieName, sess.ID, int(IdleTimeout.Seconds()), "/", "", CookieSecure(), true)

		c.Set(ctxSessionKey, sess)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		v, ok := c.Get(ctxSessionKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		sess, ok := v.(*Session)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		if _, ok := allowed[sess.RoleName]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

func CurrentSession(c *gin.Context) (*Session, bool) {
	v, ok := c.Get(ctxSessionKey)
	if !ok {
		return nil, false
	}
	sess, ok := v.(*Session)
	return sess, ok
}
