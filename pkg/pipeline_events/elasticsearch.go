package pipline_events

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"fmt"

	"bytes"

	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

type result interface {
}

type ESIssue struct {
	v1.IssueSummary
	Environment map[string]string
}

type Index struct {
	Created bool
	Id      string `json:"_id,omitempty"`
}

// ElasticsearchProvider implements PipelineEventsProvider interface for elasticsearch
type ElasticsearchProvider struct {
	Client    *http.Client
	BasicAuth string
	BaseURL   string
}

func NewElasticsearchProvider(server *auth.AuthServer, user *auth.UserAuth) (PipelineEventsProvider, error) {

	basicAuth := util.BasicAuth(user.Username, user.Password)

	provider := ElasticsearchProvider{
		BaseURL:   server.URL,
		BasicAuth: basicAuth,
		Client:    http.DefaultClient,
	}

	return &provider, nil
}

func (e ElasticsearchProvider) SendActivity(a *v1.PipelineActivity) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	var index *Index

	err = e.post("activities", string(a.UID), data, &index)
	if err != nil {
		return err
	}

	if index.Id == "" {
		return fmt.Errorf("activity %s not created, no elasticsearch id returned from POST\n", a.Name)
	}
	return nil
}

func (e ElasticsearchProvider) SendRelease(r *v1.Release) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	var index *Index

	err = e.post("releases", string(r.UID), data, &index)
	if err != nil {
		return err
	}

	if index.Id == "" {
		return fmt.Errorf("release %s not created, no elasticsearch id returned from POST\n", r.Name)
	}

	for _, i := range r.Spec.Issues {
		esissue := ESIssue{
			i, map[string]string{
				r.Namespace: r.CreationTimestamp.String(),
			},
		}
		e.SendIssue(&esissue)
	}
	return nil
}

func (e ElasticsearchProvider) SendIssue(i *ESIssue) error {
	id := strings.Replace(i.URL, ":", "-", -1)
	id = strings.Replace(id, "/", "-", -1)
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	var index *Index

	log.Logger().Infof("sending issue %s", id)
	err = e.post("issues", id, data, &index)
	if err != nil {
		return err
	}

	if index.Id == "" {
		return fmt.Errorf("issue %s not created, no elasticsearch id returned from POST\n", id)
	}

	return nil
}
func (e ElasticsearchProvider) post(index, indexID string, body []byte, rs result) error {

	url := fmt.Sprintf("%s/%s/event/%s", e.BaseURL, index, indexID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Add("Authorization", "Basic "+e.BasicAuth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return fmt.Errorf("error POSTing to elasticsearch %v", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("error response POSTing to elasticsearch: %s", resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(data, &rs)
	if err != nil {
		return fmt.Errorf("error unmarshalling %v", err)
	}
	return nil
}
