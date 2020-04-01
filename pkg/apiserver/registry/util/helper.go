package util

import (
	"context"
	configv1alpha1 "github.com/kiosk-sh/kiosk/pkg/apis/config/v1alpha1"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"github.com/kiosk-sh/kiosk/pkg/util/subject"
	"k8s.io/apiserver/pkg/authentication/user"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
)

func IsUserPartOfAccount(userInfo user.Info, account *configv1alpha1.Account) bool {
	subjects := map[string]bool{}
	subjects[constants.UserPrefix+userInfo.GetName()] = true
	for _, group := range userInfo.GetGroups() {
		subjects[constants.GroupPrefix+group] = true
	}
	for _, accountSubject := range account.Spec.Subjects {
		accountSubjectKey := subject.ConvertSubject("", &accountSubject)
		if subjects[accountSubjectKey] {
			return true
		}
	}

	return false
}

func GetAccountsByUserInfo(ctx context.Context, client client2.Client, userInfo user.Info) ([]*configv1alpha1.Account, error) {
	subjects := []string{constants.UserPrefix + userInfo.GetName()}
	for _, group := range userInfo.GetGroups() {
		subjects = append(subjects, constants.GroupPrefix+group)
	}

	retList := []*configv1alpha1.Account{}
	accList := &configv1alpha1.AccountList{}
	for _, subject := range subjects {
		err := client.List(ctx, accList, client2.MatchingFields{constants.IndexBySubjects: subject})
		if err != nil {
			return nil, err
		}
		for _, acc := range accList.Items {
			c := acc
			retList = append(retList, &c)
		}
	}

	return retList, nil
}
