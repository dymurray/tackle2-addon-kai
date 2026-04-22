package main

import (
	"os"
	"path"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-addon/ssh"
	hub "github.com/konveyor/tackle2-hub/addon"
)

var (
	addon     = hub.Addon
	Dir       = ""
	SourceDir = ""
	Source    = "Kai"
)

type Data struct {
	Repository repository.SCM
	Source     string
}

func init() {
	Dir, _ = os.Getwd()
	SourceDir = path.Join(Dir, "source")
}

func main() {
	addon.Run(func() (err error) {
		d := &Data{}
		err = addon.DataWith(d)
		if err != nil {
			return
		}
		if d.Source == "" {
			d.Source = Source
		}

		//
		// Fetch application.
		addon.Activity("Fetching application.")
		application, err := addon.Task.Application()
		if err != nil {
			return
		}

		// Pallet sync skills and log hashes
		// use archetype to get appropriate skills?
		// arch, err := addon.Task.Archetype()

		// Fetch assessment to get questions for usage in mig plan?

		// SSH agent
		agent := ssh.Agent{}
		err = agent.Start()
		if err != nil {
			return
		}
		// Fetch source repo
		err = FetchRepository(application)
		if err != nil {
			return
		}

		// Perform migration using skills and migration plan

		// Push to a branch

		// TODO: Should we use asset repo for this? or just pick a branch name and push to original source?
		return
	})
}
