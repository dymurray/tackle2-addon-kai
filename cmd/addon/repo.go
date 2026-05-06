package main

import (
	"errors"
	"path"
	"strings"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-hub/api"
)

// FetchRepository clones the application's SCM repository and returns the
// repository.SCM handle so callers can reuse it for Branch/Commit/push.
func FetchRepository(application *api.Application) (rp repository.SCM, err error) {
	if application.Repository == nil {
		err = errors.New("application repository not defined")
		return
	}
	var options []any
	identity, found, err :=
		addon.Application.Identity(application.ID).Search().
			Direct("source").
			Indirect("source").
			Find()
	if err != nil {
		return
	}
	if found {
		options = append(options, identity)
	}
	SourceDir = path.Join(
		SourceDir,
		strings.Split(
			path.Base(
				application.Repository.URL),
			".")[0])
	rp, err = repository.New(
		SourceDir,
		application.Repository,
		options...)
	if err != nil {
		return
	}
	err = rp.Fetch()
	return
}
