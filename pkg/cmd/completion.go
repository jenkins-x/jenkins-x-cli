package cmd

import (
	"bytes"
	"io"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
)

const boilerPlate = ""

var (
	completion_long = templates.LongDesc(`
		Output shell completion code for the given shell (bash or zsh).

		This command prints shell code which must be evaluation to provide interactive
		completion of jx commands.

		    $ source <(jx completion bash)

		will load the jx completion code for bash. Note that this depends on the
		bash-completion framework. It must be sourced before sourcing the jx
		completion, e.g. on the Mac:

		    $ brew install bash-completion
		    $ source $(brew --prefix)/etc/bash_completion
		    $ source <(jx completion bash)

		On a Mac it often works better to generate a file with the completion and source that:

			$ jx completion bash > ~/.jx/bash
			$ source ~/.jx/bash

		If you use zsh[1], the following will load jx zsh completion:

		    $ source <(jx completion zsh)

		[1] zsh completions are only supported in versions of zsh >= 5.2`)
)

var (
	completion_shells = map[string]func(out io.Writer, cmd *cobra.Command) error{
		"bash": runCompletionBash,
		"zsh":  runCompletionZsh,
	}
	// It is likely that the user has the completions for kubectl loaded, so reusing function from there if they exist
	bashCompletionFunctions = `
__jx_get_env() {
	local jx_out
    if jx_out=$(jx get env | tail -n +2 | cut -d' ' -f1 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${jx_out[*]}" -- "$cur" ) )
    fi
}

__jx_get_promotionstrategies() {
	COMPREPLY=( $(compgen -W "` + strings.Join(v1.PromotionStrategyTypeValues, " ") + `" -- ${cur}) )
}

__jx_custom_func() {
    case ${last_command} in
        jx_environment )
            __jx_get_env
            return
            ;;
		jx_namespace )
			declare -f __kubectl_get_resource_namespace > /dev/null && __kubectl_get_resource_namespace
			return
			;;
        *)
            ;;
    esac
}
`
)

// CompletionOptions options for completion command
type CompletionOptions struct {
	*opts.CommonOptions
}

func NewCmdCompletion(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CompletionOptions{
		CommonOptions: commonOpts,
	}

	shells := []string{}
	for s := range completion_shells {
		shells = append(shells, s)
	}

	cmd := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Output shell completion code for the given shell (bash or zsh)",
		Long:  completion_long,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		ValidArgs: shells,
	}

	return cmd
}

// Run executes the completion command
func (o *CompletionOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	shells := []string{}
	for s := range completion_shells {
		shells = append(shells, s)
	}
	var ShellName string
	cmd := o.Cmd
	args := o.Args
	if len(args) == 0 {
		prompts := &survey.Select{
			Message:  "Shell",
			Options:  shells,
			PageSize: len(shells),
			Help:     "The name of the shell",
		}
		err := survey.AskOne(prompts, &ShellName, nil, surveyOpts)
		if err != nil {
			return err
		}

	}
	if len(args) > 1 {
		return helper.UsageError(cmd, "Too many arguments. Expected only the shell type.")
	}
	if ShellName == "" {
		ShellName = args[0]
	}

	run, found := completion_shells[ShellName]

	if !found {
		return helper.UsageError(cmd, "Unsupported shell type %q.", args[0])
	}

	cmd.Parent().BashCompletionFunction = bashCompletionFunctions

	return run(o.Out, cmd.Parent())
}

func runCompletionBash(out io.Writer, cmd *cobra.Command) error {
	if boilerPlate != "" {
		_, err := out.Write([]byte(boilerPlate))
		if err != nil {
			return err
		}
	}
	return cmd.GenBashCompletion(out)
}

func runCompletionZsh(out io.Writer, cmd *cobra.Command) error {
	zsh_head := "#compdef jx\n"

	out.Write([]byte(zsh_head))

	if boilerPlate != "" {
		_, err := out.Write([]byte(boilerPlate))
		if err != nil {
			return err
		}
	}
	zsh_initialization := `
__jx_bash_source() {
	alias shopt=':'
	alias _expand=_bash_expand
	alias _complete=_bash_comp
	emulate -L sh
	setopt kshglob noshglob braceexpand
	source "$@"
}
__jx_type() {
	# -t is not supported by zsh
	if [ "$1" == "-t" ]; then
		shift
		# fake Bash 4 to disable "complete -o nospace". Instead
		# "compopt +-o nospace" is used in the code to toggle trailing
		# spaces. We don't support that, but leave trailing spaces on
		# all the time
		if [ "$1" = "__jx_compopt" ]; then
			echo builtin
			return 0
		fi
	fi
	type "$@"
}
__jx_compgen() {
	local completions w
	completions=( $(compgen "$@") ) || return $?
	# filter by given word as prefix
	while [[ "$1" = -* && "$1" != -- ]]; do
		shift
		shift
	done
	if [[ "$1" == -- ]]; then
		shift
	fi
	for w in "${completions[@]}"; do
		if [[ "${w}" = "$1"* ]]; then
			echo "${w}"
		fi
	done
}
__jx_compopt() {
	true # don't do anything. Not supported by bashcompinit in zsh
}
__jx_ltrim_colon_completions()
{
	if [[ "$1" == *:* && "$COMP_WORDBREAKS" == *:* ]]; then
		# Remove colon-word prefix from COMPREPLY items
		local colon_word=${1%${1##*:}}
		local i=${#COMPREPLY[*]}
		while [[ $((--i)) -ge 0 ]]; do
			COMPREPLY[$i]=${COMPREPLY[$i]#"$colon_word"}
		done
	fi
}
__jx_get_comp_words_by_ref() {
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[${COMP_CWORD}-1]}"
	words=("${COMP_WORDS[@]}")
	cword=("${COMP_CWORD[@]}")
}
__jx_filedir() {
	local RET OLD_IFS w qw
	__jx_debug "_filedir $@ cur=$cur"
	if [[ "$1" = \~* ]]; then
		# somehow does not work. Maybe, zsh does not call this at all
		eval echo "$1"
		return 0
	fi
	OLD_IFS="$IFS"
	IFS=$'\n'
	if [ "$1" = "-d" ]; then
		shift
		RET=( $(compgen -d) )
	else
		RET=( $(compgen -f) )
	fi
	IFS="$OLD_IFS"
	IFS="," __jx_debug "RET=${RET[@]} len=${#RET[@]}"
	for w in ${RET[@]}; do
		if [[ ! "${w}" = "${cur}"* ]]; then
			continue
		fi
		if eval "[[ \"\${w}\" = *.$1 || -d \"\${w}\" ]]"; then
			qw="$(__jx_quote "${w}")"
			if [ -d "${w}" ]; then
				COMPREPLY+=("${qw}/")
			else
				COMPREPLY+=("${qw}")
			fi
		fi
	done
}
__jx_quote() {
    if [[ $1 == \'* || $1 == \"* ]]; then
        # Leave out first character
        printf %q "${1:1}"
    else
    	printf %q "$1"
    fi
}
autoload -U +X bashcompinit && bashcompinit
# use word boundary patterns for BSD or GNU sed
LWORD='[[:<:]]'
RWORD='[[:>:]]'
if sed --help 2>&1 | grep -q GNU; then
	LWORD='\<'
	RWORD='\>'
fi
__jx_convert_bash_to_zsh() {
	sed \
	-e 's/declare -F/whence -w/' \
	-e 's/_get_comp_words_by_ref "\$@"/_get_comp_words_by_ref "\$*"/' \
	-e 's/local \([a-zA-Z0-9_]*\)=/local \1; \1=/' \
	-e 's/flags+=("\(--.*\)=")/flags+=("\1"); two_word_flags+=("\1")/' \
	-e 's/must_have_one_flag+=("\(--.*\)=")/must_have_one_flag+=("\1")/' \
	-e "s/${LWORD}_filedir${RWORD}/__jx_filedir/g" \
	-e "s/${LWORD}_get_comp_words_by_ref${RWORD}/__jx_get_comp_words_by_ref/g" \
	-e "s/${LWORD}__ltrim_colon_completions${RWORD}/__jx_ltrim_colon_completions/g" \
	-e "s/${LWORD}compgen${RWORD}/__jx_compgen/g" \
	-e "s/${LWORD}compopt${RWORD}/__jx_compopt/g" \
	-e "s/${LWORD}declare${RWORD}/builtin declare/g" \
	-e "s/\\\$(type${RWORD}/\$(__jx_type/g" \
	<<'BASH_COMPLETION_EOF'
`
	out.Write([]byte(zsh_initialization))

	buf := new(bytes.Buffer)
	cmd.GenBashCompletion(buf)
	out.Write(buf.Bytes())

	zsh_tail := `
BASH_COMPLETION_EOF
}
__jx_bash_source <(__jx_convert_bash_to_zsh)
_complete jx 2>/dev/null
`
	out.Write([]byte(zsh_tail))
	return nil
}
