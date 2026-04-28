package main

import (
	"errors"
	"path"
	"strings"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-hub/api"
)

// FetchRepository gets SCM repository.
func FetchRepository(application *api.Application) (err error) {
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
	var rp repository.SCM
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
