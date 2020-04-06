package create

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/kube/services"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	optionName = "name"
)

var (
	create_etc_hosts_long = templates.LongDesc(`
		Creates /etc/hosts entries for all current exposed services
`)

	create_etc_hosts_example = templates.Examples(`
		# Creates /etc/hosts entries for all current exposed services
		sudo jx create etc-hosts
	`)
)

// CreateEtcHostsOptions the options for the create spring command
type CreateEtcHostsOptions struct {
	options.CreateOptions

	Name string
	IP   string
}

// NewCmdCreateEtcHosts creates a command object for the "create" command
func NewCmdCreateEtcHosts(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateEtcHostsOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "etc-hosts kind [url]",
		Short:   "Creates a new Git server URL",
		Aliases: []string{"etchosts", "etc_hosts"},
		Long:    create_etc_hosts_long,
		Example: create_etc_hosts_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Name, optionName, "n", "/etc/hosts", "The etc hosts file to edit")
	cmd.Flags().StringVarP(&options.IP, "ip", "i", "", "The IP address of the node to point the host entries to")
	return cmd
}

// Run implements the command
func (o *CreateEtcHostsOptions) Run() error {
	name := o.Name
	if name == "" {
		return util.MissingOption(name)
	}
	client, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	urls, err := services.FindServiceURLs(client, ns)
	if err != nil {
		return err
	}
	exists, err := util.FileExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("hosts file %s does not exist!", name)
	}
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	idx, ipLine := o.findIPLine(&lines)
	for _, u := range urls {
		ipLine = o.addUrl(u, ipLine)
	}
	lines[idx] = ipLine
	newText := strings.Join(lines, "\n")
	if newText != text {
		err = ioutil.WriteFile(name, []byte(newText), util.DefaultWritePermissions)
		if err != nil {
			return err
		}
		log.Logger().Infof("Updated file %s", util.ColorInfo(name))
	}
	return nil
}

func (o *CreateEtcHostsOptions) addUrl(serviceUrl services.ServiceURL, ipLine string) string {
	text := serviceUrl.URL
	u, err := url.Parse(text)
	if err != nil {
		log.Logger().Warnf("Ignored invalid URL %s %s", text, err)
		return ipLine
	}
	host := u.Host
	fields := strings.Fields(ipLine)
	for i := 1; i < len(fields); i++ {
		if fields[i] == host {
			return ipLine
		}
	}
	if !strings.HasSuffix(ipLine, " ") {
		ipLine += " "
	}
	return ipLine + host
}

func (o *CreateEtcHostsOptions) findIPLine(lines *[]string) (int, string) {
	prefix := o.IP + " "
	for i, line := range *lines {
		if strings.HasPrefix(line, prefix) {
			return i, line
		}
	}

	idx := len(*lines) + 2
	*lines = append(*lines, "", "# jx added services entries", prefix)
	return idx, prefix

}
