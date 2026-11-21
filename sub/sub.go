package sub

import (
	"strings"

	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http/httpguts"
)

type customHeader struct {
	key   string
	value string
}

// RegisterRoutes mounts the subscription endpoints onto the panel's router.
// Panel and subscriptions now share a single listener, so they also share one
// port, TLS configuration and domain. Callers pass the panel engine's root
// group; the configured subPath is applied here.
func RegisterRoutes(g *gin.RouterGroup) error {
	s := &service.SettingService{}

	customHeadersStr, err := s.GetSubCustomHeaders()
	if err != nil {
		return err
	}
	if customHeaders := parseCustomHeaders(customHeadersStr); len(customHeaders) > 0 {
		g.Use(func(c *gin.Context) {
			for _, h := range customHeaders {
				c.Writer.Header().Set(h.key, h.value)
			}
			c.Next()
		})
	}

	subPath, err := s.GetSubPath()
	if err != nil {
		return err
	}

	NewSubHandler(g.Group(subPath))
	return nil
}

func parseCustomHeaders(raw string) []customHeader {
	var headers []customHeader
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" || !httpguts.ValidHeaderFieldName(key) || !httpguts.ValidHeaderFieldValue(value) {
			continue
		}
		headers = append(headers, customHeader{key: key, value: value})
	}
	return headers
}
