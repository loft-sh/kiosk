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

package controllers

import (
	"fmt"
	"github.com/loft-sh/kiosk/pkg/util/loghelper"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Register registers the webhooks to the manager
func Register(mgr manager.Manager) error {
	err := (&AccountReconciler{
		Client: mgr.GetClient(),
		Log:    loghelper.New("account-controller"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	if err != nil {
		return fmt.Errorf("Unable to create account controller: %v", err)
	}

	err = (&TemplateInstanceReconciler{
		Client: mgr.GetClient(),
		Log:    loghelper.New("template-instance-controller"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	if err != nil {
		return fmt.Errorf("Unable to create template instance controller: %v", err)
	}

	return nil
}
