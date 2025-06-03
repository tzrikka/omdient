package links

import (
	"github.com/tzrikka/omdient/pkg/links/receivers"
	"github.com/tzrikka/omdient/pkg/links/slack"
)

// WebhookHandlers is a map of all the link-specific webhooks that Omdient supports.
var WebhookHandlers = map[string]receivers.WebhookHandlerFunc{
	"slack-bot-token": slack.WebhookHandler,
}
