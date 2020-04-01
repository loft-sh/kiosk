/*
Copyright 2020 DevSpace Technologies Inc.

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

package events

import (
	"github.com/go-logr/logr"
	"github.com/kiosk-sh/kiosk/pkg/constants"

	"github.com/kiosk-sh/kiosk/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NamespaceEventHandler handles events
type NamespaceEventHandler struct {
	Log logr.Logger
}

// Create implements EventHandler
func (e *NamespaceEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

// Update implements EventHandler
func (e *NamespaceEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	if util.GetAccountFromNamespace(evt.MetaOld) == util.GetAccountFromNamespace(evt.MetaNew) {
		return
	}

	e.handleEvent(evt.MetaOld, q)
	e.handleEvent(evt.MetaNew, q)
}

// Delete implements EventHandler
func (e *NamespaceEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

// Generic implements EventHandler
func (e *NamespaceEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.handleEvent(evt.Meta, q)
}

func (e *NamespaceEventHandler) handleEvent(meta metav1.Object, q workqueue.RateLimitingInterface) {
	if meta == nil {
		return
	}

	labels := meta.GetLabels()
	if labels == nil {
		return
	}

	if owner, ok := labels[constants.SpaceLabelAccount]; ok && owner != "" {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name: owner,
		}})
	}
}
