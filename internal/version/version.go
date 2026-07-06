package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "0.1.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Channel   string `json:"channel"`
}

func Get() Info {
	channel := "dev"
	if Version != "" && Version != "0.1.0-dev" {
		channel = "stable"
	}
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Channel:   channel,
	}
}

func (i Info) String() string {
	return fmt.Sprintf("viewred %s (%s/%s) commit=%s built=%s",
		i.Version, i.OS, i.Arch, i.Commit, i.BuildDate)
}

func (i Info) UserAgent() string {
	return fmt.Sprintf("viewred/%s (%s; %s)", i.Version, i.OS, i.Arch)
}
