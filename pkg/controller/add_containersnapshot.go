package controller

import (
	"github.com/supremind/container-snapshot/pkg/controller/containersnapshot"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, containersnapshot.Add)
}
