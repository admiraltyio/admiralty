/*
Copyright 2018 The Multicluster-Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manager // import "admiralty.io/multicluster-controller/pkg/manager"

import (
	"fmt"
	"sync"
)

// ControllerSet is a set of Controllers.
type ControllerSet map[Controller]struct{}

// CacheSet is a set of Caches.
type CacheSet map[Cache]struct{}

// Manager manages controllers. It starts their caches, waits for those to sync, then starts the controllers.
type Manager struct {
	controllers ControllerSet
}

// New creates a Manager.
func New() *Manager {
	return &Manager{controllers: make(ControllerSet)}
}

// Cache is the interface used by Manager to start and wait for caches to sync.
type Cache interface {
	Start(stop <-chan struct{}) error
	WaitForCacheSync(stop <-chan struct{}) bool
}

// Controller is the interface used by Manager to start the controllers and get their caches (beforehand).
type Controller interface {
	Start(stop <-chan struct{}) error
	GetCaches() CacheSet
}

// AddController adds a controller to the Manager.
func (m *Manager) AddController(c Controller) {
	m.controllers[c] = struct{}{}
}

// Start gets all the unique caches of the controllers it manages, starts them,
// then starts the controllers as soon as their respective caches are synced.
// Start blocks until an error or stop is received.
func (m *Manager) Start(stop <-chan struct{}) error {
	errCh := make(chan error)

	wgs := make(map[Controller]*sync.WaitGroup)
	caches := make(map[Cache]ControllerSet)

	for co := range m.controllers {
		wgs[co] = &sync.WaitGroup{}
		for ca := range co.GetCaches() {
			wgs[co].Add(1)
			cos, ok := caches[ca]
			if !ok {
				cos = make(ControllerSet)
				caches[ca] = cos
			}
			cos[co] = struct{}{}
		}
	}

	for ca, cos := range caches {
		go func(ca Cache) {
			if err := ca.Start(stop); err != nil {
				errCh <- err
			}
		}(ca)
		go func(ca Cache, cos ControllerSet) {
			if ok := ca.WaitForCacheSync(stop); !ok {
				errCh <- fmt.Errorf("failed to wait for caches to sync")
			}
			for co := range cos {
				wgs[co].Done()
			}
		}(ca, cos)
	}

	for co := range m.controllers {
		go func(co Controller) {
			wgs[co].Wait()
			if err := co.Start(stop); err != nil {
				errCh <- err
			}
		}(co)
	}

	select {
	case <-stop:
		return nil
	case err := <-errCh:
		return err
	}
}
