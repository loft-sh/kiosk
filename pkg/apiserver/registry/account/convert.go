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

package account

import (
	"encoding/json"

	config "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	tenancyv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/tenancy/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertConfigAccount converts a config account into a tenancy account
func ConvertConfigAccount(configAccount *config.Account) (*tenancy.Account, error) {
	out, err := json.Marshal(configAccount)
	if err != nil {
		return nil, err
	}

	outAccount := &tenancyv1alpha1.Account{}
	err = json.Unmarshal(out, outAccount)
	if err != nil {
		return nil, err
	}

	outAccount.TypeMeta = metav1.TypeMeta{
		Kind:       "Account",
		APIVersion: tenancyv1alpha1.SchemeGroupVersion.String(),
	}
	outAccount.ObjectMeta = *configAccount.ObjectMeta.DeepCopy()

	tenancyAccount := &tenancy.Account{}
	err = tenancyv1alpha1.Convert_v1alpha1_Account_To_tenancy_Account(outAccount, tenancyAccount, nil)
	return tenancyAccount, err
}

// ConvertTenancyAccount converts a tenancy account into a config account
func ConvertTenancyAccount(originalAccount *tenancy.Account) (*config.Account, error) {
	tenancyAccount := &tenancyv1alpha1.Account{}
	err := tenancyv1alpha1.Convert_tenancy_Account_To_v1alpha1_Account(originalAccount, tenancyAccount, nil)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(tenancyAccount)
	if err != nil {
		return nil, err
	}

	outAccount := &config.Account{}
	err = json.Unmarshal(out, outAccount)
	if err != nil {
		return nil, err
	}

	outAccount.ObjectMeta = *tenancyAccount.ObjectMeta.DeepCopy()
	return outAccount, nil
}
