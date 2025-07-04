package links

import (
	"github.com/tzrikka/omdient/internal/links"
	"github.com/tzrikka/omdient/pkg/links/github"
	"github.com/tzrikka/omdient/pkg/links/slack"
)

// WebhookHandlers is a map of all the link-specific
// stateless webhook handlers that Omdient supports.
var WebhookHandlers = map[string]links.WebhookHandlerFunc{
	"github-app-jwt":  github.WebhookHandler,
	"github-user-pat": github.WebhookHandler,
	"github-webhook":  github.WebhookHandler,
	"slack-bot-token": slack.WebhookHandler,
	"slack-oauth":     slack.WebhookHandler,
	"slack-oauth-gov": slack.WebhookHandler,
}

// ConnectionHandlers is a map of all the link-specific
// stateful connection handlers that Omdient supports.
var ConnectionHandlers = map[string]links.ConnectionHandlerFunc{
	"slack-socket-mode": slack.ConnectionHandler,
}
