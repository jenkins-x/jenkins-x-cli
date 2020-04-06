package chats

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/util"
)

// ChatProvider represents an integration interface to chat
type ChatProvider interface {
	GetChannelMetrics(name string) (*ChannelMetrics, error)
}

// ChannelMetrics metrics for a channel
type ChannelMetrics struct {
	ID          string
	Name        string
	URL         string
	MemberCount int
	Members     []string
}

func (m *ChannelMetrics) ToMarkdown() string {
	return util.MarkdownLink(m.Name, m.URL)
}

// CreateChatProvider creates a new chat provider if one is available for the given kind
func CreateChatProvider(kind string, server *auth.AuthServer, userAuth *auth.UserAuth, batchMode bool) (ChatProvider, error) {
	switch kind {
	case Slack:
		return CreateSlackChatProvider(server, userAuth, batchMode)
	default:
		return nil, fmt.Errorf("Unsupported chat provider kind: %s", kind)
	}
}

func ProviderAccessTokenURL(kind string, url string) string {
	switch kind {
	case Slack:
		return "https://my.slack.com/services/new/bot"
	default:
		return ""
	}
}
