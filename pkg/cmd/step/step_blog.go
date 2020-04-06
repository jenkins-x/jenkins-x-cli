package step

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	releases2 "github.com/jenkins-x/jx/pkg/gits/releases"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/chats"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/issues"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/reports"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	stepBlogLong = templates.LongDesc(`
		Generates charts for a project
`)

	stepBlogExample = templates.Examples(`
		# create charts for the cuect
		jx step chart

			`)

	ignoreNewUsers = map[string]bool{
		"GitHub": true,
	}
)

// StepBlogOptions contains the command line flags
type StepBlogOptions struct {
	step.StepOptions

	FromDate                    string
	ToDate                      string
	Dir                         string
	BlogOutputDir               string
	BlogName                    string
	CombineMinorReleases        bool
	DeveloperChannelMemberCount int
	UserChannelMemberCount      int

	State StepBlogState
}

type StepBlogState struct {
	GitInfo                  *gits.GitRepository
	GitProvider              gits.GitProvider
	Tracker                  issues.IssueProvider
	Release                  *v1.Release
	BlogFileName             string
	DeveloperChatMetricsName string
	UserChatMetricsName      string
	Buffer                   *bytes.Buffer
	Writer                   *bufio.Writer
	HistoryService           *reports.ProjectHistoryService
	History                  *reports.ProjectHistory
	NewContributors          map[string]*v1.UserDetails
	NewCommitters            map[string]*v1.UserDetails
}

// NewCmdStepBlog Creates a new Command object
func NewCmdStepBlog(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepBlogOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "blog",
		Short:   "Creates a blog post with changes, metrics and charts showing improvements",
		Long:    stepBlogLong,
		Example: stepBlogExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The directory to query to find the projects .git directory")
	cmd.Flags().StringVarP(&options.FromDate, "from-date", "f", "", "The date to create the charts from. Defaults to a week before the to date. Should be a format: "+util.DateFormat)
	cmd.Flags().StringVarP(&options.ToDate, "to-date", "t", "", "The date to query up to. Defaults to now. Should be a format: "+util.DateFormat)
	cmd.Flags().StringVarP(&options.BlogOutputDir, "blog-dir", "", "", "The Hugo-style blog source code to generate the charts into")
	cmd.Flags().StringVarP(&options.BlogName, "blog-name", "n", "", "The blog name")
	cmd.Flags().BoolVarP(&options.CombineMinorReleases, "combine-minor", "c", true, "If enabled lets combine minor releases together to simplify the charts")
	cmd.Flags().IntVarP(&options.DeveloperChannelMemberCount, "dev-channel-members", "", 0, "If no chat bots can connect to your chat server you can pass in the counts for the developer channel here")
	cmd.Flags().IntVarP(&options.UserChannelMemberCount, "user-channel-members", "", 0, "If no chat bots can connect to your chat server you can pass in the counts for the user channel here")
	return cmd
}

// Run implements this command
func (o *StepBlogOptions) Run() error {
	o.State = StepBlogState{
		NewContributors: map[string]*v1.UserDetails{},
		NewCommitters:   map[string]*v1.UserDetails{},
	}
	pc, _, err := config.LoadProjectConfig(o.Dir)
	if err != nil {
		return err
	}
	outDir := o.BlogOutputDir
	if outDir != "" {
		if o.BlogName == "" {
			t := time.Now()
			o.BlogName = "changes-" + strconv.Itoa(t.Day()) + "-" + strings.ToLower(t.Month().String()) + "-" + strconv.Itoa(t.Year())
		}
		historyFile := filepath.Join(o.BlogOutputDir, "data", "projectHistory.yml")
		o.State.HistoryService, o.State.History, err = reports.NewProjectHistoryService(historyFile)
		if err != nil {
			return err
		}

		err = o.generateChangelog()
		if err != nil {
			return err
		}
	} else {
		gitInfo, gitProvider, tracker, err := o.CreateGitProvider(o.Dir)
		if err != nil {
			return err
		}
		if gitInfo == nil {
			return fmt.Errorf("Could not find a .git folder in the current directory so could not determine the current project")
		}
		o.State.GitInfo = gitInfo
		o.State.GitProvider = gitProvider
		o.State.Tracker = tracker
	}

	err = o.downloadsReport(o.State.GitProvider, o.State.GitInfo.Organisation, o.State.GitInfo.Name)
	if err != nil {
		return err
	}
	if pc.Chat != nil {
		err = o.loadChatMetrics(pc.Chat)
		if err != nil {
			return err
		}
	}
	return o.addReportsToBlog()
}

func (o *StepBlogOptions) downloadsReport(provider gits.GitProvider, owner string, repo string) error {
	releases, err := provider.ListReleases(owner, repo)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		log.Logger().Warnf("No releases found for %s/%s/n", owner, repo)
		return nil
	}
	if o.CombineMinorReleases {
		releases = o.combineMinorReleases(releases)
	}
	release := o.State.Release
	history := o.State.History
	if history != nil {
		history.DownloadMetrics(o.ToDate, releases2.ReleaseDownloadCount(releases))
		if release != nil {
			spec := &release.Spec
			issuesClosed := len(spec.Issues)
			queryClosedIssueCount, err := o.queryClosedIssues()
			if err != nil {
				log.Logger().Warnf("Failed to query closed issues: %s", err)
			}
			if queryClosedIssueCount > issuesClosed {
				issuesClosed = queryClosedIssueCount
			}
			history.IssueMetrics(o.ToDate, issuesClosed)
			history.PullRequestMetrics(o.ToDate, len(spec.PullRequests))
			history.CommitMetrics(o.ToDate, len(spec.Commits))
		}

		repo, err := provider.GetRepository(owner, repo)
		if err != nil {
			log.Logger().Warnf("Failed to load the repository %s", err)
		} else {
			history.StarsMetrics(o.ToDate, repo.Stars)
		}
	}

	report := o.createBarReport("downloads", "Version", "Downloads")

	for _, release := range releases {
		report.AddNumber(release.Name, release.DownloadCount)
	}
	return report.Render()
}

// createBarReport creates the new report instance
func (o *StepBlogOptions) createBarReport(name string, legends ...string) reports.BarReport {
	outDir := o.BlogOutputDir
	if outDir != "" {
		blogName := o.BlogName
		if blogName == "" {
			t := time.Now()
			blogName = fmt.Sprintf("changes-%d-%s-%d", t.Day(), t.Month().String(), t.Year())
		}

		jsDir := filepath.Join(outDir, "static", "news", blogName)
		err := os.MkdirAll(jsDir, util.DefaultWritePermissions)
		if err != nil {
			log.Logger().Warnf("Could not create directory %s: %s", jsDir, err)
		}
		jsFileName := filepath.Join(jsDir, name+".js")
		jsLinkURI := filepath.Join("/news", blogName, name+".js")
		state := &o.State
		if state.Buffer == nil {
			var buffer bytes.Buffer
			state.Buffer = &buffer
		}
		if state.Writer == nil {
			state.Writer = bufio.NewWriter(state.Buffer)
		}
		state.Buffer.WriteString(`

## ` + strings.Title(name) + `

`)
		return reports.NewBlogBarReport(name, state.Writer, jsFileName, jsLinkURI)
	}
	return reports.NewTableBarReport(o.CreateTable(), legends...)
}

func (options *StepBlogOptions) combineMinorReleases(releases []*gits.GitRelease) []*gits.GitRelease {
	answer := []*gits.GitRelease{}
	m := map[string]*gits.GitRelease{}
	for _, release := range releases {
		name := release.Name
		if name != "" {
			idx := strings.LastIndex(name, ".")
			if idx > 0 {
				name = name[0:idx] + ".x"
			}
		}
		cur := m[name]
		if cur == nil {
			copy := *release
			copy.Name = name
			m[name] = &copy
			answer = append(answer, &copy)
		} else {
			cur.DownloadCount += release.DownloadCount
		}
	}
	return answer
}

func (o *StepBlogOptions) generateChangelog() error {
	blogFile := filepath.Join(o.BlogOutputDir, "content", "news", o.BlogName+".md")
	previousDate := o.FromDate
	now := time.Now()
	if previousDate == "" {
		// default to 4 weeks ago
		t := now.Add(-time.Hour * 24 * 7 * 4)
		previousDate = util.FormatDate(t)
		o.FromDate = previousDate
	}
	if o.ToDate == "" {
		o.ToDate = util.FormatDate(now)
	}
	options := &StepChangelogOptions{
		StepOptions: o.StepOptions,
		Dir:         o.Dir,
		Version:     "Changes since " + previousDate,
		// TODO add time now and previous time
		PreviousDate:       previousDate,
		OutputMarkdownFile: blogFile,
	}
	err := options.Run()
	if err != nil {
		return err
	}
	state := &o.State
	output := &options.State
	state.GitInfo = output.GitInfo
	state.GitProvider = output.GitProvider
	state.Tracker = output.Tracker
	state.Release = output.Release
	state.BlogFileName = blogFile
	return nil
}

func (o *StepBlogOptions) addReportsToBlog() error {
	state := &o.State
	if state.BlogFileName != "" {
		data, err := ioutil.ReadFile(state.BlogFileName)
		if err != nil {
			return err
		}
		toDate := o.ToDate
		fromDate := o.FromDate
		committersText := o.createNewCommitters()

		prefix := `---
title: "Changes for ` + toDate + `"
date: ` + time.Now().Format(time.RFC3339) + `
description: "Whats new for ` + toDate + `"
categories: [blog]
keywords: []
slug: "changes-` + strings.Replace(toDate, " ", "-", -1) + `"
aliases: []
author: jenkins-x-bot
---

## Changes for ` + toDate + `

This blog outlines the changes on the project from ` + fromDate + ` to ` + toDate + `.

` + o.createMetricsSummary() + `

[View Charts](#charts)

` + committersText

		postfix := ""
		if state.Writer != nil {
			state.Writer.Flush()
			postfix = `

## Charts

` + state.Buffer.String() + `

This blog post was generated via the [jx step blog](https://jenkins-x.io/commands/jx_step_blog/) command from [Jenkins X](https://jenkins-x.io/).
`

		}
		changelog := strings.TrimSpace(string(data))
		changelog = strings.TrimPrefix(changelog, "## Changes")
		text := prefix + changelog + postfix
		err = ioutil.WriteFile(state.BlogFileName, []byte(text), util.DefaultWritePermissions)
		if err != nil {
			return err
		}
	}
	historyService := state.HistoryService
	if historyService != nil {
		return historyService.SaveHistory()
	}
	return nil
}

func (o *StepBlogOptions) createMetricsSummary() string {
	var buffer bytes.Buffer
	out := bufio.NewWriter(&buffer)
	_, report := o.report()
	if report != nil {
		developerChatMetricsName := o.State.DeveloperChatMetricsName
		if developerChatMetricsName == "" {
			developerChatMetricsName = "Developer Chat Members"
		}
		userChatMetricsName := o.State.UserChatMetricsName
		if userChatMetricsName == "" {
			userChatMetricsName = "User Chat Members"
		}

		fmt.Fprintf(out, "| Metrics     | Changes | Total |\n")
		fmt.Fprintf(out, "| :---------- | -------:| -----:|\n")
		o.printMetrics(out, "Downloads", &report.DownloadMetrics)
		o.printMetrics(out, "Stars", &report.StarsMetrics)
		o.printMetrics(out, "New Committers", &report.NewCommitterMetrics)
		o.printMetrics(out, "New Contributors", &report.NewContributorMetrics)
		o.printMetrics(out, developerChatMetricsName, &report.DeveloperChatMetrics)
		o.printMetrics(out, userChatMetricsName, &report.UserChatMetrics)
		o.printMetrics(out, "Issues Closed", &report.IssueMetrics)
		o.printMetrics(out, "Pull Requests Merged", &report.PullRequestMetrics)
		o.printMetrics(out, "Commits", &report.CommitMetrics)
	}
	out.Flush()
	return buffer.String()
}

func (o *StepBlogOptions) report() (*reports.ProjectHistory, *reports.ProjectReport) {
	history := o.State.History
	if history != nil {
		toDate := o.ToDate
		report := history.FindReport(toDate)
		if report == nil {
			log.Logger().Warnf("No report for date %s", toDate)
		}
		return history, report
	}
	return nil, nil
}

func (o *StepBlogOptions) printMetrics(out io.Writer, name string, metrics *reports.CountMetrics) {
	count := metrics.Count
	total := metrics.Total
	if count > 0 || total > 0 {
		fmt.Fprintf(out, "| %s | **%d** | **%d** |\n", name, count, total)
	}
}

func (o *StepBlogOptions) createNewCommitters() string {
	release := o.State.Release
	if release != nil {
		spec := &release.Spec
		// TODO commits typically don't have a login so lets ignore for now
		// and assume that they show up in the PRs
		/*
			for _, commit := range spec.Commits {
				o.addCommitter(commit.Committer)
				o.addCommitter(commit.Author)
			}
		*/
		for _, pr := range spec.PullRequests {
			o.addCommitter(pr.User)
			o.addCommitters(pr.Assignees)
		}
		for _, issue := range spec.Issues {
			o.addContributor(issue.User)
			o.addContributors(issue.Assignees)
		}
	} else {
		log.Logger().Warnf("No Release!")
	}
	history, _ := o.report()
	if history != nil {
		history.NewContributorMetrics(o.ToDate, len(o.State.NewContributors))
		history.NewCommitterMetrics(o.ToDate, len(o.State.NewCommitters))

		// now lets remove the current contributors
		for _, user := range history.Committers {
			delete(o.State.NewCommitters, user)
		}
		for _, user := range history.Contributors {
			delete(o.State.NewContributors, user)
		}

		// now lets add the new users to the history for the next blog
		for _, user := range o.State.NewCommitters {
			history.Committers = append(history.Committers, user.Login)
		}
		for _, user := range o.State.NewContributors {
			history.Contributors = append(history.Contributors, user.Login)
		}
	}

	var buffer bytes.Buffer
	out := bufio.NewWriter(&buffer)
	o.printUserMap(out, "committers", o.State.NewCommitters)
	o.printUserMap(out, "contributors", o.State.NewContributors)
	out.Flush()
	return buffer.String()
}

func (o *StepBlogOptions) printUserMap(out io.Writer, role string, newUsers map[string]*v1.UserDetails) {
	if len(newUsers) > 0 {
		out.Write([]byte(`

## New ` + strings.Title(role) + `

Welcome to our new ` + role + `!

`))

		keys := []string{}
		for k := range newUsers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			user := newUsers[k]
			if user != nil {
				out.Write([]byte("* " + o.formatUser(user) + "\n"))
			}
		}
	}
}

func (o *StepBlogOptions) addCommitters(users []v1.UserDetails) {
	for _, u := range users {
		o.addCommitter(&u)
	}
}

func (o *StepBlogOptions) addContributors(users []v1.UserDetails) {
	for _, u := range users {
		o.addContributor(&u)
	}
}

func (o *StepBlogOptions) addContributor(user *v1.UserDetails) {
	o.addUser(user, &o.State.NewContributors)
}

func (o *StepBlogOptions) addCommitter(user *v1.UserDetails) {
	o.addUser(user, &o.State.NewCommitters)
	o.addContributor(user)
}

func (o *StepBlogOptions) addUser(user *v1.UserDetails, newUsers *map[string]*v1.UserDetails) {
	if user != nil {
		key := user.Login
		if key == "" {
			key = user.Name
		}
		oldUser := (*newUsers)[key]
		if key != "" && !ignoreNewUsers[key] && oldUser == nil || user.URL != "" {
			(*newUsers)[key] = user
		}
	}
}

func (o *StepBlogOptions) formatUser(user *v1.UserDetails) string {
	u := user.URL
	login := user.Login
	if u == "" {
		u = util.UrlJoin(o.State.GitProvider.ServerURL(), login)
	}
	name := user.Name
	if name == "" {
		name = login
	}
	if name == "" {
		name = u
	}
	if name == "" {
		return ""
	}
	prefix := ""
	avatar := user.AvatarURL
	if avatar != "" {
		prefix = "<img class='avatar' src='" + avatar + "' height='32' width='32'> "
	}
	label := prefix + name
	if u != "" {
		return "<a href='" + u + "' title='" + user.Name + "'>" + label + "</a>"
	}
	return label
}

func (o *StepBlogOptions) loadChatMetrics(chatConfig *config.ChatConfig) error {
	u := chatConfig.URL
	if u == "" {
		return nil
	}
	history, _ := o.report()
	if history == nil {
		return nil
	}
	const membersPostfix = " members"
	devChannel := chatConfig.DeveloperChannel
	if devChannel != "" {
		count := o.DeveloperChannelMemberCount
		if count > 0 {
			o.State.DeveloperChatMetricsName = devChannel + membersPostfix
		} else {
			metrics, err := o.getChannelMetrics(chatConfig, devChannel)
			if err != nil {
				log.Logger().Warnf("Failed to get chat metrics for channel %s: %s", devChannel, err)
				return nil
			}
			count = metrics.MemberCount
			o.State.DeveloperChatMetricsName = metrics.ToMarkdown() + membersPostfix
		}
		history.DeveloperChatMetrics(o.ToDate, count)
	}
	userChannel := chatConfig.UserChannel
	if userChannel != "" {
		count := o.UserChannelMemberCount
		if count > 0 {
			o.State.UserChatMetricsName = userChannel + membersPostfix
		} else {
			metrics, err := o.getChannelMetrics(chatConfig, userChannel)
			if err != nil {
				log.Logger().Warnf("Failed to get chat metrics for channel %s: %s", userChannel, err)
				return nil
			}
			count = metrics.MemberCount
			o.State.UserChatMetricsName = metrics.ToMarkdown() + membersPostfix
		}
		history.UserChatMetrics(o.ToDate, count)
	}
	return nil
}

func (o *StepBlogOptions) getChannelMetrics(chatConfig *config.ChatConfig, channelName string) (*chats.ChannelMetrics, error) {
	provider, err := o.CreateChatProvider(chatConfig)
	if err != nil {
		return nil, err
	}
	return provider.GetChannelMetrics(channelName)
}

func (o *StepBlogOptions) queryClosedIssues() (int, error) {
	fromDate := o.FromDate
	if fromDate == "" {
		return 0, fmt.Errorf("No from date specified!")
	}
	t, err := util.ParseDate(fromDate)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse from date: %s: %s", fromDate, err)
	}
	issues, err := o.State.Tracker.SearchIssuesClosedSince(t)
	count := len(issues)
	return count, err
}

// CreateChatProvider creates a new chart provider from the given configuration
func (o *StepBlogOptions) CreateChatProvider(chatConfig *config.ChatConfig) (chats.ChatProvider, error) {
	u := chatConfig.URL
	if u == "" {
		return nil, nil
	}
	authConfigSvc, err := o.CreateChatAuthConfigService("")
	if err != nil {
		return nil, err
	}
	config := authConfigSvc.Config()

	server := config.GetOrCreateServer(u)
	userAuth, err := config.PickServerUserAuth(server, "user to access the chat service at "+u, o.BatchMode, "", o.GetIOFileHandles())
	if err != nil {
		return nil, err
	}
	return chats.CreateChatProvider(server.Kind, server, userAuth, o.BatchMode)
}
