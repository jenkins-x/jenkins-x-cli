package opts

import "github.com/spf13/cobra"

//CommonDevPodOptions are common flags that are to be applied across all DevPod commands
type CommonDevPodOptions struct {
	Username string
}

// AddCommonDevPodFlags adds the dev pod common flags to the given cobra command
func (o *CommonDevPodOptions) AddCommonDevPodFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Username, "username", "", "", "The username to create the DevPod. If not specified defaults to the current operating system user or $USER'")
}
