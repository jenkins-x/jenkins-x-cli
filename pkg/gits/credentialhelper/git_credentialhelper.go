package credentialhelper

import (
	"bufio"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

// GitCredentialsHelper is used to implement the git credential helper algorithm.
// See also https://git-scm.com/docs/git-credential.
type GitCredentialsHelper struct {
	in               io.Reader
	out              io.Writer
	knownCredentials []GitCredential
}

// CreateGitCredentialsHelper creates an instance of a git credential helper. It needs to get passed the handles to read
// the git credential data as well as write the response to. It also gets the list og known credentials.
func CreateGitCredentialsHelper(in io.Reader, out io.Writer, credentials []GitCredential) (*GitCredentialsHelper, error) {
	if in == nil {
		return nil, errors.New("in parameter cannot be nil")
	}

	if out == nil {
		return nil, errors.New("out parameter cannot be nil")
	}

	if credentials == nil {
		return nil, errors.New("credentials parameter cannot be nil")
	}

	return &GitCredentialsHelper{
		in:               in,
		out:              out,
		knownCredentials: credentials,
	}, nil
}

// Run executes the specified git credential helper operation which must be one of get, store or erase.
// NOTE: Currently only get is implemented.
func (h *GitCredentialsHelper) Run(op string) error {
	var err error

	switch op {
	case "get":
		err = h.Get()
	case "store":
		// not yet implemented (HF)
		fmt.Println("")
	case "erase":
		// not yet implemented (HF)
		fmt.Println("")
	default:
		err = errors.Errorf("invalid git credential operation '%s'", op)
	}

	return err
}

// Get implements the get operation of the git credential helper protocol. It reads the authentication query from
// the reader of this instance and writes the response to the writer (usually this will be stdout and stdin).
func (h *GitCredentialsHelper) Get() error {
	var data []string
	scanner := bufio.NewScanner(h.in)
	for scanner.Scan() {
		data = append(data, scanner.Text())
	}

	if scanner.Err() != nil {
		return errors.Wrap(scanner.Err(), "unable to read input from stdin")
	}

	gitCredential, err := CreateGitCredential(data)
	if err != nil {
		return errors.Wrap(scanner.Err(), "unable to create GitCredential struct")
	}

	answer := h.Fill(gitCredential)

	_, err = fmt.Fprintf(h.out, answer.String())
	if err != nil {
		return errors.Wrap(err, "unable to write response to stdin")
	}

	return nil
}

// Fill creates a GitCredential instance based on a git credential helper request which represented by the passed queryCredential instance.
// If there is no auth information available an empty credential instance is returned
func (h *GitCredentialsHelper) Fill(queryCredential GitCredential) GitCredential {
	for _, authCredential := range h.knownCredentials {
		if queryCredential.Protocol != authCredential.Protocol {
			continue
		}

		if queryCredential.Host != authCredential.Host {
			continue
		}

		if queryCredential.Path != authCredential.Path {
			continue
		}

		answer := authCredential.Clone()
		return answer
	}

	return GitCredential{}
}
