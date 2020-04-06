package chats

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/nlopes/slack"
)

type SlackChatProvider struct {
	SlackClient *slack.Client
	Server      *auth.AuthServer
	UserAuth    *auth.UserAuth
}

func CreateSlackChatProvider(server *auth.AuthServer, userAuth *auth.UserAuth, batchMode bool) (ChatProvider, error) {
	u := server.URL
	if u == "" {
		return nil, fmt.Errorf("No base URL for server!")
	}
	if userAuth == nil || userAuth.IsInvalid() || userAuth.ApiToken == "" {
		return nil, fmt.Errorf("No authentication found for Slack server %s", u)
	}
	slackClient := slack.New(userAuth.ApiToken)

	return &SlackChatProvider{
		SlackClient: slackClient,
		Server:      server,
		UserAuth:    userAuth,
	}, nil
}

func (c *SlackChatProvider) GetChannelMetrics(name string) (*ChannelMetrics, error) {
	metrics := &ChannelMetrics{
		Name: name,
	}
	name = strings.TrimPrefix(name, "#")
	id := name

	channels, err := c.SlackClient.GetChannels(true)
	if err != nil {
		return metrics, err
	}
	for _, ch := range channels {
		log.Logger().Infof("Found channel %s with id %s", ch.Name, ch.ID)
		if ch.Name == name {
			id = ch.ID
			break
		}
	}
	info, err := c.SlackClient.GetChannelInfo(id)
	if err != nil {
		return metrics, err
	}
	metrics.MemberCount = info.NumMembers
	metrics.ID = info.ID
	metrics.Name = info.Name
	metrics.Members = info.Members
	metrics.URL = util.UrlJoin(c.Server.URL, "messages", info.ID)
	return metrics, nil
}
