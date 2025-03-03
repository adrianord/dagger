package gitdns

import (
	"path"

	"github.com/dagger/dagger/engine/buildkit"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/moby/buildkit/util/sshutil"
	"github.com/pkg/errors"
)

const AttrNetConfig = "gitdns.netconfig"

// Git is a helper mimicking the llb.Git function, but with the ability to
// set additional attributes.
func State(url, ref string, clientIDs []string, opts ...llb.GitOption) llb.State {
	hi := &llb.GitInfo{}
	for _, o := range opts {
		o.SetGitOption(hi)
	}

	remote, err := gitutil.ParseURL(url)
	if errors.Is(err, gitutil.ErrUnknownProtocol) {
		url = "https://" + url
		remote, err = gitutil.ParseURL(url)
	}
	if remote != nil {
		remote.Fragment = ""
		url = remote.String()
	}

	var id string
	if err != nil {
		// If we can't parse the URL, just use the full URL as the ID. The git
		// operation will fail later on.
		id = url
	} else {
		// We construct the ID manually here, so that we can create the same ID
		// for different protocols (e.g. https and ssh) that have the same
		// host/path/fragment combination.
		id = remote.Host + path.Join("/", remote.Path)
		if ref != "" {
			id += "#" + ref
		}
	}
	attrs := map[string]string{}

	// TODO(vito): replace when custom sources are supported
	hack, err := buildkit.EncodeIDHack(DaggerGitURLHack{
		Remote:    url,
		ClientIDs: clientIDs,
	})
	if err != nil {
		panic(err)
	}
	url = "git://" + hack

	gi := &llb.GitInfo{
		AuthHeaderSecret: "GIT_AUTH_HEADER",
		AuthTokenSecret:  "GIT_AUTH_TOKEN",
	}
	for _, o := range opts {
		o.SetGitOption(gi)
	}
	if gi.KeepGitDir {
		attrs[pb.AttrKeepGitDir] = "true"
	}
	if url != "" {
		attrs[pb.AttrFullRemoteURL] = url
	}
	if gi.AuthTokenSecret != "" {
		attrs[pb.AttrAuthTokenSecret] = gi.AuthTokenSecret
	}
	if gi.AuthHeaderSecret != "" {
		attrs[pb.AttrAuthHeaderSecret] = gi.AuthHeaderSecret
	}
	if remote != nil && remote.Scheme == gitutil.SSHProtocol {
		if gi.KnownSSHHosts != "" {
			attrs[pb.AttrKnownSSHHosts] = gi.KnownSSHHosts
		} else {
			keyscan, err := sshutil.SSHKeyScan(remote.Host)
			if err == nil {
				// best effort
				attrs[pb.AttrKnownSSHHosts] = keyscan
			}
		}

		if gi.MountSSHSock == "" {
			attrs[pb.AttrMountSSHSock] = "default"
		} else {
			attrs[pb.AttrMountSSHSock] = gi.MountSSHSock
		}
	}

	source := llb.NewSource("git://"+id, attrs, gi.Constraints)
	return llb.NewState(source.Output())
}
